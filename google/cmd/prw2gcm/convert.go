// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	monitoring_pb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	timestamp_pb "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/prometheus/client_golang/exp/api/remote"
	writev2 "github.com/prometheus/prometheus/google/cmd/prw2gcm/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/model/value"
	distribution_pb "google.golang.org/genproto/googleapis/api/distribution"
	metric_pb "google.golang.org/genproto/googleapis/api/metric"
	monitoredres_pb "google.golang.org/genproto/googleapis/api/monitoredres"
)

// The target label keys used for the Prometheus monitored resource.
const (
	KeyProjectID = "project_id"
	KeyLocation  = "location"
	KeyCluster   = "cluster"
	KeyNamespace = "namespace"
	KeyJob       = "job"
	KeyInstance  = "instance"

	// Maximum number of labels allowed on GCM series.
	maxLabelCount = 100

	metricTypePrefix = "prometheus.googleapis.com"
)

type gcmQueue interface {
	Enqueue(ts *monitoring_pb.TimeSeries)
	Flush()
}

// Convert converts Remote Write 2.x series and enqueue them as GCM v3 time series.
// TODO: Given this proxy is meant to be mostly for testing or temporary use, the implementation is limited. Consider extending if needed:
// * Support for classic histograms (accumulation needed, but possible).
//
// Other limitations, not planned to be fixed here for now:
// * No support for no CT for cumulative.
// For the general GCM implementation, those limitations might need to be handled.
func Convert(ctx context.Context, r *writev2.Request, q gcmQueue) (stats remote.WriteResponseStats, retErr error) {
	initialSeries := make([]*monitoring_pb.TimeSeries, 0, len(r.Timeseries))
	initialSeriesTsIndex := make([]int, 0, len(r.Timeseries))
	maxPoints := 0

	// Do initial pass for initial series to send.
	for i, ts := range r.Timeseries {
		if ctx.Err() != nil {
			retErr = httpErrJoin(retErr, ctx.Err())
			return stats, retErr
		}

		exportTS, points, err := initialTimeSeriesConvert(ts, r.Symbols)
		if err != nil {
			retErr = httpErrJoin(retErr, fmt.Errorf("series %v: initial conversion failed; series skipped: %w", ts.LabelsToString(r.Symbols), err))
			continue
		}
		if maxPoints < points {
			maxPoints = points
		}
		initialSeries = append(initialSeries, exportTS)
		initialSeriesTsIndex = append(initialSeriesTsIndex, i)
	}

	// Go through samples. We have to do it over sample dimension, not series, given the
	// GCM CreateTimeSeries API constraint of having one point for a single timeseries per whole request.
	// NOTE: Prometheus currently only sends 1 sample per request per timeseries, but we are implementing the protocol here
	// which supports future extensions (buffering) on Prometheus ecosystem side.
	for p := 0; p < maxPoints; p++ {
		for i, exportTS := range initialSeries {
			if ctx.Err() != nil {
				retErr = httpErrJoin(retErr, ctx.Err())
				return stats, retErr
			}

			ts := r.Timeseries[initialSeriesTsIndex[i]]
			s, point, err := convertTimeSeriesPoint(exportTS, ts, p)
			if err != nil {
				retErr = httpErrJoin(retErr, fmt.Errorf("series %v: point conversion failed; series skipped: %w", ts.LabelsToString(r.Symbols), err))
				continue
			}
			if point == nil {
				continue
			}

			// GCM supports single sample point only, reuse some parts, but generally send another timeseries.
			// Shallow copy is enough (we care about the points being different only).
			exportTSCopy := *exportTS
			exportTSCopy.Points = []*monitoring_pb.Point{point}
			q.Enqueue(&exportTSCopy)

			// We can't really tell the true "written" number in this proxy (this could
			// be done internally on GCM ingestion one day). Notably, even if we batch
			// fails, it could be whole batch or one sample per batch. Assume scheduled export
			// a "written" event.
			stats.Samples += s.Samples
			stats.Histograms += s.Histograms
			stats.Exemplars += s.Exemplars
		}
		// We have to flush now, so a new GCM request is ensured.
		q.Flush()
	}
	return stats, retErr
}

func initialTimeSeriesConvert(ts *writev2.TimeSeries, symbols []string) (_ *monitoring_pb.TimeSeries, points int, _ error) {
	meta := ts.GetMetadata()
	if meta == nil {
		return nil, 0, newHTTPErrorf(http.StatusBadRequest, "metadata is required")
	}
	exportTS, err := initTSFromLabels(meta, ts.LabelsRefs, symbols)
	if err != nil {
		return nil, 0, err
	}
	// Reject classic histograms for now. It's implementable (best-effort), TBD later on in GCM only.
	if meta.GetType() == writev2.Metadata_METRIC_TYPE_HISTOGRAM && len(ts.Samples) > 0 {
		// Classic histogram detected, currently, this implementation requires "self-contained-histograms" (nhcb),
		// reject classic ones. It's implementable (best-effort), TBD later on in GCM only.
		// See: https://docs.google.com/document/d/1mpcSWH1B82q-BtJza-eJ8xMLlKt6EJ9oFGH325vtY1Q/edit
		return nil, 0, newHTTPErrorf(http.StatusBadRequest, "classic histograms are not supported; use nhcb instead")
	}
	if len(ts.Samples) > 0 && len(ts.Histograms) > 0 {
		return nil, 0, newHTTPErrorf(http.StatusBadRequest, "both samples and histogram samples provided; forbidden in PRW 2.x")
	}
	switch ts.GetMetadata().GetType() {
	case writev2.Metadata_METRIC_TYPE_HISTOGRAM, writev2.Metadata_METRIC_TYPE_GAUGEHISTOGRAM:
		// Process native histogram samples.
		if len(ts.Histograms) == 0 {
			return nil, 0, newHTTPErrorf(http.StatusBadRequest, "no histogram sample provided for histogram type metric")
		}
		exportTS.ValueType = metric_pb.MetricDescriptor_DISTRIBUTION
		points = len(ts.Histograms)
	default:
		// Process float samples.
		if len(ts.Samples) == 0 {
			return nil, 0, newHTTPErrorf(http.StatusBadRequest, "no sample provided")
		}
		exportTS.ValueType = metric_pb.MetricDescriptor_DOUBLE
		points = len(ts.Samples)
	}
	return exportTS, points, nil
}

func convertTimeSeriesPoint(exportTS *monitoring_pb.TimeSeries, ts *writev2.TimeSeries, point int) (stats remote.WriteResponseStats, _ *monitoring_pb.Point, _ error) {
	switch ts.GetMetadata().GetType() {
	case writev2.Metadata_METRIC_TYPE_HISTOGRAM, writev2.Metadata_METRIC_TYPE_GAUGEHISTOGRAM:
		if len(ts.Histograms) < point {
			return stats, nil, nil
		}
		h := ts.Histograms[point]
		// TODO: Skip staleness markers.
		if exportTS.GetMetricKind() == metric_pb.MetricDescriptor_CUMULATIVE && h.CreatedTimestamp == 0 {
			return stats, nil, newHTTPErrorf(http.StatusBadRequest, "created timestamp is required for every cumulative metric")
		}
		d, err := histogramSampleToDistribution(h)
		if err != nil {
			return stats, nil, err
		}
		stats.Histograms++
		// TODO(bwplotka): Exemplars.
		return stats, &monitoring_pb.Point{
			Interval: &monitoring_pb.TimeInterval{
				StartTime: getTimestamp(h.CreatedTimestamp),
				EndTime:   getTimestamp(h.Timestamp),
			},
			Value: &monitoring_pb.TypedValue{
				Value: &monitoring_pb.TypedValue_DistributionValue{DistributionValue: d},
			},
		}, nil
	default:
		if len(ts.Samples) < point {
			return stats, nil, nil
		}
		s := ts.Samples[point]
		if value.IsStaleNaN(s.Value) {
			// Staleness markers are currently unsupported.
			return stats, nil, nil
		}

		if exportTS.GetMetricKind() == metric_pb.MetricDescriptor_CUMULATIVE && s.CreatedTimestamp == 0 {
			return stats, nil, newHTTPErrorf(http.StatusBadRequest, "created timestamp is required for every cumulative metric")
		}
		stats.Samples++
		// TODO(bwplotka): Exemplars.
		return stats, &monitoring_pb.Point{
			Interval: &monitoring_pb.TimeInterval{
				StartTime: getTimestamp(s.CreatedTimestamp),
				EndTime:   getTimestamp(s.Timestamp),
			},
			Value: &monitoring_pb.TypedValue{
				Value: &monitoring_pb.TypedValue_DoubleValue{DoubleValue: s.Value},
			},
		}, nil
	}
}

func initTSFromLabels(meta *writev2.Metadata, labelsRefs []uint32, symbols []string) (*monitoring_pb.TimeSeries, error) {
	name := ""
	resLabels := map[string]string{}
	metricLabels := map[string]string{}

	// Remote Write contains all labels in one sorted, interned array.
	// Validate if we have all labels required for the resource.
	// TODO(bwplotka): Check len(labelRefs) mod 2, etc
	for i := 0; i < len(labelsRefs); i += 2 {
		n := symbols[labelsRefs[i]]
		v := symbols[labelsRefs[i+1]]

		switch n {
		case "__name__":
			if v == "" {
				return nil, newHTTPErrorf(http.StatusBadRequest, "empty metric name (__name__) label")
			}
			name = v
		case KeyProjectID, KeyLocation, KeyCluster, KeyNamespace, KeyJob, KeyInstance:
			if v == "" {
				continue
			}
			resLabels[n] = v
		default:
			metricLabels[n] = v
		}
	}
	if name == "" {
		return nil, newHTTPErrorf(http.StatusBadRequest, "no metric name (__name__) label found")
	}
	if len(metricLabels) > maxLabelCount {
		// TODO: Is the field limit is lifted in the GCM API already?
		return nil, newHTTPErrorf(http.StatusBadRequest, "metric labels exceed the limit of %d; got %v", maxLabelCount, len(metricLabels))
	}

	descriptor, kind, err := describeMetric(name, meta.GetType())
	if err != nil {
		return nil, err
	}
	res := &monitoredres_pb.MonitoredResource{
		Type: "prometheus_target",
		Labels: map[string]string{
			// Ensure all required labels are set (even if empty), otherwise GCM request
			// will fail. Empty string is a valid value in GCM and not the same as being unset.
			// NOTE: We could consider mode of the proxy where we validate those. Ignore for now.
			KeyProjectID: resLabels[KeyProjectID],
			KeyLocation:  resLabels[KeyLocation],
			KeyCluster:   resLabels[KeyCluster],
			KeyNamespace: resLabels[KeyNamespace],
			KeyJob:       resLabels[KeyJob],
			KeyInstance:  resLabels[KeyInstance],
		},
	}
	return &monitoring_pb.TimeSeries{
		Resource: res,
		Metric: &metric_pb.Metric{
			Type:   descriptor,
			Labels: metricLabels,
		},
		MetricKind:  kind,
		Description: symbols[meta.GetHelpRef()],
		Unit:        symbols[meta.GetUnitRef()], // TODO: Check if it's safe to do (new behaviour vs export pkg).
	}, nil
}

// getTimestamp converts a millisecond timestamp into a protobuf timestamp.
func getTimestamp(t int64) *timestamp_pb.Timestamp {
	return &timestamp_pb.Timestamp{
		Seconds: t / 1000,
		Nanos:   int32((t % 1000) * int64(time.Millisecond)),
	}
}

func histogramSampleToDistribution(s *writev2.Histogram) (*distribution_pb.Distribution, error) {
	// TODO(bwplotka): implement.
	return nil, errors.New("exponential histogram not implemented yet")

}

// describeMetric creates a GCM metric type from the Prometheus metric name and a type suffix.
// Optionally, a secondary type suffix may be provided for series for which a Prometheus type
// may be written as different GCM series.
//
// The general rule is that if the primary suffix is ambiguous about whether the specific series
// is to be treated as a counter or gauge at query time, the secondarySuffix is set to "counter"
// for the counter variant, and left empty for the gauge variant.
func describeMetric(name string, typ writev2.Metadata_MetricType) (
		descriptor string,
		kind metric_pb.MetricDescriptor_MetricKind,
		_ error,
) {
	suffix := gcmMetricSuffixNone
	extraSuffix := gcmMetricSuffixNone

	switch typ {
	case writev2.Metadata_METRIC_TYPE_COUNTER:
		suffix = gcmMetricSuffixCounter
		kind = metric_pb.MetricDescriptor_CUMULATIVE
	case writev2.Metadata_METRIC_TYPE_GAUGE:
		suffix = gcmMetricSuffixGauge
		kind = metric_pb.MetricDescriptor_GAUGE
	case writev2.Metadata_METRIC_TYPE_HISTOGRAM:
		suffix = gcmMetricSuffixHistogram
		kind = metric_pb.MetricDescriptor_CUMULATIVE
	case writev2.Metadata_METRIC_TYPE_GAUGEHISTOGRAM:
		// TODO: Check if it's safe to do (new behaviour vs export pkg).
		suffix = gcmMetricSuffixHistogram
		kind = metric_pb.MetricDescriptor_GAUGE
	case writev2.Metadata_METRIC_TYPE_SUMMARY:
		switch ms := getMetricSuffix(name); ms {
		case metricSuffixSum:
			suffix = gcmMetricSuffixSummary
			extraSuffix = gcmMetricSuffixCounter
			kind = metric_pb.MetricDescriptor_CUMULATIVE
		case metricSuffixCount:
			suffix = gcmMetricSuffixSummary
			extraSuffix = gcmMetricSuffixNone
			kind = metric_pb.MetricDescriptor_CUMULATIVE
		case metricSuffixNone: // Actual quantiles.
			suffix = gcmMetricSuffixSummary
			extraSuffix = gcmMetricSuffixNone
			kind = metric_pb.MetricDescriptor_GAUGE
		default:
			return "", kind, newHTTPErrorf(http.StatusBadRequest, "unknown summary series suffix %v", ms)
		}
	case writev2.Metadata_METRIC_TYPE_INFO, writev2.Metadata_METRIC_TYPE_STATESET:
		// TODO: Check if it's safe to do (new behaviour vs export pkg).
		suffix = gcmMetricSuffixGauge
		kind = metric_pb.MetricDescriptor_GAUGE
	case writev2.Metadata_METRIC_TYPE_UNSPECIFIED:
		fallthrough
	default:
		return "", kind, newHTTPErrorf(http.StatusBadRequest, "unknown metric type %v", typ)
	}
	if extraSuffix == gcmMetricSuffixNone {
		return fmt.Sprintf("%s/%s/%s", metricTypePrefix, name, suffix), kind, nil
	}
	return fmt.Sprintf("%s/%s/%s:%s", metricTypePrefix, name, suffix, extraSuffix), kind, nil
}

func getMetricSuffix(name string) metricSuffix {
	if strings.HasSuffix(name, string(metricSuffixTotal)) {
		return metricSuffixTotal
	}
	if strings.HasSuffix(name, string(metricSuffixBucket)) {
		return metricSuffixBucket
	}
	if strings.HasSuffix(name, string(metricSuffixCount)) {
		return metricSuffixCount
	}
	if strings.HasSuffix(name, string(metricSuffixSum)) {
		return metricSuffixSum
	}
	return metricSuffixNone
}

// Metric name suffixes used by various Prometheus metric types.
type metricSuffix string

const (
	metricSuffixNone   metricSuffix = ""
	metricSuffixTotal  metricSuffix = "_total"
	metricSuffixBucket metricSuffix = "_bucket"
	metricSuffixSum    metricSuffix = "_sum"
	metricSuffixCount  metricSuffix = "_count"
)

// Suffixes appended to GCM metric types. They are equivalent to the respective
// Prometheus types but we redefine them here to ensure they don't unexpectedly change
// by updating a Prometheus library.
type gcmMetricSuffix string

const (
	gcmMetricSuffixNone      gcmMetricSuffix = ""
	gcmMetricSuffixUnknown   gcmMetricSuffix = "unknown"
	gcmMetricSuffixGauge     gcmMetricSuffix = "gauge"
	gcmMetricSuffixCounter   gcmMetricSuffix = "counter"
	gcmMetricSuffixHistogram gcmMetricSuffix = "histogram"
	gcmMetricSuffixSummary   gcmMetricSuffix = "summary"
)
