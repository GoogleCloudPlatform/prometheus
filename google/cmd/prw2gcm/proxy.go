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
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync"

	"cloud.google.com/go/auth"
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	monitoring_pb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/client_golang/prometheus"
	writev2 "github.com/prometheus/prometheus/google/cmd/prw2gcm/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/google/export"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

type proxy struct {
	opts   proxyOpts
	client *monitoring.MetricClient
}

type proxyOpts struct {
	logger           *slog.Logger
	endpoint         string
	defaultUserAgent string

	// credentialsFile is a path with the Google service account JSON file to use
	// for GCM gRPC calls. If no credentials are set, proxy will try to deduce default credentials.
	credentialsFile string

	// forwardCredentials enables mode where proxy will
	// expect an HTTP 'Authorization' header with the Bearer token in the incoming
	// Remote Write 2.x requests. This token will be then forwarded to the gRPC GCM unary call.
	// This mode allows running this proxy as a Prometheus sidecar, using Prometheus remote_write
	// auth setup. This flow is not uncommon e.g. see https://github.com/grpc-ecosystem/grpc-gateway/blob/2fba1914fcc12696707a5dfa91dbf92cdb7af555/runtime/context.go#L136
	// IMPORTANT: For this mode proxy, in the insecure environments, proxy
	// should be served behind TLS.
	forwardCredentials bool
}

var (
	_ auth.TokenProvider = &perRequestAuthProvider{}
)

type requestAuthTokenCtxKeyType struct{}

var (
	requestAuthTokenCtxKey requestAuthTokenCtxKeyType
)

// perRequestAuthProvider allows providing access auth token on every gRPC call,
// if the given context has a token under requestAuthTokenCtxKey.
// If no token is found in the context, error is returned, causing gRPC call to fail.
type perRequestAuthProvider struct{}

func (p *perRequestAuthProvider) Token(ctx context.Context) (*auth.Token, error) {
	token, ok := ctx.Value(requestAuthTokenCtxKey).(string)
	if !ok {
		// Should never happen as we ensure this value exists on proxy.Store method.
		return nil, errors.New("token not found in the context")
	}
	return &auth.Token{Value: token}, nil
}

func newProxy(ctx context.Context, opts proxyOpts) (_ *proxy, err error) {
	p := &proxy{
		opts: opts,
	}

	// Setup GCM client.
	clientOpts := []option.ClientOption{
		option.WithGRPCDialOption(grpc.WithUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor)),
	}
	if opts.defaultUserAgent != "" {
		clientOpts = append(clientOpts, option.WithUserAgent(opts.defaultUserAgent))
	}
	if opts.endpoint != "" {
		clientOpts = append(clientOpts, option.WithEndpoint(opts.endpoint))
	}
	if opts.credentialsFile != "" {
		if opts.forwardCredentials {
			return nil, errors.New("both credentialsFile and forwardCredentials options set on proxy; choose one")
		}
		slog.Info("using JSON file credentials provided")
		clientOpts = append(clientOpts, option.WithCredentialsFile(opts.credentialsFile))
	}
	if opts.forwardCredentials {
		slog.Info("setting up flow for using the Bearer Authorization token from the incoming HTTP request")
		clientOpts = append(clientOpts, option.WithAuthCredentials(
			auth.NewCredentials(&auth.CredentialsOptions{TokenProvider: &perRequestAuthProvider{}}),
		))
	}
	p.client, err = monitoring.NewMetricClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("create GCM client: %w", err)
	}
	return p, nil
}

func (p *proxy) Handler(reg prometheus.Registerer) (pattern string, handler http.Handler) {
	return pathPrefix, detectPOSTMethodProjectID(
		instrument(reg, path.Join(pathPrefix, "PROJECT_ID", pathSuffix),
			remote.NewWriteHandler(
				p,
				remote.MessageTypes{remote.WriteV2MessageType},
				remote.WithWriteHandlerLogger(p.opts.logger),
			),
		),
	)
}

// Copied from https://github.com/grpc-ecosystem/grpc-gateway/blob/2fba1914fcc12696707a5dfa91dbf92cdb7af555/runtime/context.go#L123
func isValidGRPCMetadataTextValue(textValue string) bool {
	// Must be a valid gRPC "ASCII-Value" as defined here:
	//   https://github.com/grpc/grpc/blob/4b05dc88b724214d0c725c8e7442cbc7a61b1374/doc/PROTOCOL-HTTP2.md
	// This means printable ASCII (including/plus spaces); 0x20 to 0x7E inclusive.
	bytes := []byte(textValue) // gRPC validates strings on the byte level, not Unicode.
	for _, ch := range bytes {
		if ch < 0x20 || ch > 0x7E {
			return false
		}
	}
	return true
}

func (p *proxy) injectAuthFromRequest(req *http.Request) (context.Context, error) {
	// Forward potential auth token from the client.
	authToken := req.Header.Get("Authorization")
	if authToken == "" {
		return nil, newHTTPError(http.StatusUnauthorized, "no Authorization header found")
	}
	// We expect bearer token, find the token itself.
	parts := strings.Split(authToken, " ")
	if len(parts) != 2 {
		return nil, newHTTPErrorf(http.StatusUnauthorized, "unexpected format of the Authorization header value, found %v part for ' ' delimiter; expected 'Bearer <token>' format", len(parts))
	}
	if !isValidGRPCMetadataTextValue(parts[1]) {
		return nil, newHTTPError(http.StatusBadRequest, "value of HTTP Authorization header contains non-ASCII value (not valid as gRPC metadata)")
	}
	return context.WithValue(req.Context(), requestAuthTokenCtxKey, parts[1]), nil
}

// Store is what's invoked from client_golang's remote_write handling library when 2.x Remote Write is sent.
// It parses PRW 2.x and sends GCM CreateTimesSeries (max) 200 series batches for every set of samples.
func (p *proxy) Store(req *http.Request, _ remote.WriteMessageType) (_ *remote.WriteResponse, retErr error) {
	w := remote.NewWriteResponse()

	ctx := req.Context()
	if p.opts.forwardCredentials {
		var err error
		ctx, err = p.injectAuthFromRequest(req)
		if err != nil {
			w.SetStatusCode(httpCodeFromErrorOr500(err))
			return w, err
		}
	}
	projectID := getProjectID(req.Context())
	if projectID == "unknown" {
		// Programmatic error, should not happen if the detectPOSTMethodProjectID middleware is set.
		return nil, newHTTPError(http.StatusInternalServerError, "no project id found")
	}

	serializedRequest, err := io.ReadAll(req.Body)
	if err != nil {
		w.SetStatusCode(http.StatusBadRequest)
		return w, err
	}

	r := &writev2.Request{}
	if err := r.UnmarshalVT(serializedRequest); err != nil {
		w.SetStatusCode(http.StatusInternalServerError)
		return w, fmt.Errorf("decoding v2 request %w", err)
	}

	if len(r.Timeseries) == 0 {
		w.SetStatusCode(http.StatusBadRequest)
		return w, errors.New("no series in v2 request")
	}

	qm := startQueueManager(ctx, func(ctx context.Context, series []*monitoring_pb.TimeSeries) error {
		if err := p.client.CreateTimeSeries(
			ctx,
			&monitoring_pb.CreateTimeSeriesRequest{Name: fmt.Sprintf("projects/%s", projectID), TimeSeries: series},
		); err != nil {
			return newHTTPErrorFromGRPC(fmt.Errorf("gcm batch send failed for %v series; no more retries; %w", len(series), err))
		}
		return nil
	})
	defer func() {
		qm.CloseAndWait()
		retErr = httpErrJoin(retErr, qm.Err())
		if retErr != nil {
			w.SetStatusCode(httpCodeFromErrorOr500(retErr))
		}
	}()

	stats, err := Convert(ctx, r, qm)
	w.Samples += stats.Samples
	w.Histograms += stats.Histograms
	w.Exemplars += stats.Exemplars
	return w, err
}

type gcmCreateTimeSeriesFunc func(ctx context.Context, series []*monitoring_pb.TimeSeries) error

type queueManager struct {
	ctx           context.Context
	queueCh       chan *monitoring_pb.TimeSeries
	gcmCreateTSFn gcmCreateTimeSeriesFunc

	wg  sync.WaitGroup
	err error
}

func startQueueManager(ctx context.Context, gcmCreateTSFn gcmCreateTimeSeriesFunc) *queueManager {
	q := &queueManager{
		ctx:           ctx,
		queueCh:       make(chan *monitoring_pb.TimeSeries, 10),
		gcmCreateTSFn: gcmCreateTSFn,
	}
	q.wg.Add(1)

	go q.run()
	return q
}

const maxBatchSize = export.BatchSizeMax

func (q *queueManager) run() {
	defer q.wg.Done()

	batch := make([]*monitoring_pb.TimeSeries, 0, maxBatchSize)
	for {
		flushBatch := false

		// Ignore checking context, we expect close method to tell us when to stop.
		ts, ok := <-q.queueCh
		if ts == nil || !ok {
			// Sentinel value for flushing or closing channel.
			flushBatch = len(batch) > 0
		} else {
			// Adding to batch.
			batch = append(batch, ts)
			// https://cloud.google.com/monitoring/api/ref_v3/rpc/google.monitoring.v3 200 objects per request max.
			flushBatch = len(batch) >= maxBatchSize
		}

		if flushBatch {
			q.err = httpErrJoin(q.err, q.gcmCreateTSFn(q.ctx, batch))
			batch = batch[:0]
		}

		if !ok {
			return
		}
	}
}

func (q *queueManager) Enqueue(ts *monitoring_pb.TimeSeries) {
	if ts == nil {
		return
	}
	q.queueCh <- ts
}

func (q *queueManager) Flush() {
	q.queueCh <- nil
}

func (q *queueManager) CloseAndWait() {
	q.Flush()
	close(q.queueCh)
	q.wg.Wait()
}

func (q *queueManager) Err() error {
	return q.err
}
