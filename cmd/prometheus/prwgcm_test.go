package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/util/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func TestRWtoGCM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	gcmSA := os.Getenv("GCM_SECRET")
	if gcmSA == "" {
		t.Skip("skipping as GCM_SECRET= env var is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	creds, err := google.CredentialsFromJSON(ctx, []byte(gcmSA), "https://www.googleapis.com/auth/monitoring")
	require.NoError(t, err)

	tmpDir := t.TempDir()
	credsFile := filepath.Join(tmpDir, "gcm-sa.json")
	require.NoError(t, os.WriteFile(credsFile, []byte(gcmSA), 0600))

	cluster := "pe-github-action"
	location := "europe-west3-a"
	collector := "prwgcm-test"

	port := testutil.RandomUnprivilegedPort(t)

	// OpenMetrics 1
	om1Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/openmetrics-text; version=1.0.0; charset=utf-8")
		w.Write([]byte(`# TYPE test_gauge gauge
test_gauge 1.5
# TYPE test_histogram histogram
test_histogram_bucket{le="1"} 1
test_histogram_bucket{le="+Inf"} 2
test_histogram_sum 2
test_histogram_count 2
# TYPE test_summary summary
test_summary{quantile="0.5"} 1
test_summary_sum 1
test_summary_count 1
# TYPE test_stateset stateset
test_stateset{test_stateset="a"} 1
test_stateset{test_stateset="b"} 0
# TYPE test_info info
test_info_info{foo="bar"} 1
# EOF`))
	}))
	defer om1Server.Close()
	u1, _ := url.Parse(om1Server.URL)

	// Proto
	reg := prometheus.NewRegistry()
	h := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:                        "test_native_histogram",
		Help:                        "A native histogram.",
		NativeHistogramBucketFactor: 1.1,
	})
	reg.MustRegister(h)
	h.Observe(2.5)
	protoServer := httptest.NewServer(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	defer protoServer.Close()
	u2, _ := url.Parse(protoServer.URL)

	// OpenMetrics 2, for NHCB.
	om2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/openmetrics-text; version=1.0.0; charset=utf-8")
		w.Write([]byte(`# TYPE test_nhcb_histogram histogram
test_nhcb_histogram_bucket{le="1"} 1
test_nhcb_histogram_bucket{le="+Inf"} 2
test_nhcb_histogram_sum 2
test_nhcb_histogram_count 2
# EOF`))
	}))
	defer om2Server.Close()
	u3, _ := url.Parse(om2Server.URL)

	config := fmt.Sprintf(`
global:
  scrape_interval: 2s
  scrape_timeout: 2s
  external_labels:
    collector: %s
    project_id: %s
    location: %s
    cluster: %s
scrape_configs:
- job_name: 'om1'
  scrape_interval: 2s
  scrape_timeout: 2s
  static_configs:
  - targets: ['%s']
- job_name: 'proto'
  scrape_interval: 2s
  scrape_timeout: 2s
  static_configs:
  - targets: ['%s']
- job_name: 'om2'
  scrape_interval: 2s
  scrape_timeout: 2s
  convert_classic_histograms_to_nhcb: true
  static_configs:
  - targets: ['%s']
remote_write:
- name: "google_cloud"
  url: "https://staging-monitoring.sandbox.googleapis.com/v1/prometheus/api/v1/write"
  protobuf_message: "io.prometheus.write.v2.Request"
  failed_request_logging: true
  send_exemplars: true
  queue_config:
    retry_on_http_429: true
  google_iam:
    credentials_file: "%s"
`, collector, creds.ProjectID, location, cluster, u1.Host, u2.Host, u3.Host, credsFile)

	configFile := filepath.Join(tmpDir, "prometheus.yml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))

	prom := prometheusCommandWithLogging(
		t,
		configFile,
		port,
		"--storage.tsdb.path="+filepath.Join(tmpDir, "data"),
		"--enable-feature=exemplar-storage",
		"--enable-feature=native-histograms",
		"--enable-feature=promql-nhcb-as-classic",
		"--enable-feature=st-storage",
		"--enable-feature=st-synthesis",
		"--enable-feature=type-and-unit-labels",
		//"--enable-feature=metadata-wal-records",
		"--enable-feature=xor2-encoding",
	)

	require.NoError(t, prom.Start())
	t.Cleanup(func() {
		if err := prom.Process.Kill(); err != nil {
			t.Logf("Failed to kill the process: %v", err)
		}
	})

	require.Eventually(t, func() bool {
		r, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/-/ready", port))
		if err != nil {
			return false
		}
		defer r.Body.Close()
		return r.StatusCode == http.StatusOK
	}, startupTime, 100*time.Millisecond)

	// Queries
	gcmCl, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("https://staging-monitoring.sandbox.googleapis.com/v1/projects/%s/location/global/prometheus", creds.ProjectID),
		Client:  oauth2.NewClient(ctx, creds.TokenSource),
	})
	require.NoError(t, err)
	gcmAPI := v1.NewAPI(gcmCl)

	queries := []string{
		`test_gauge{job="om1"}`,
		`test_histogram_count{job="om1"}`,
		`test_summary_count{job="om1"}`,
		`test_stateset{job="om1"}`,
		`test_info_info{job="om1"}`,
		`test_native_histogram_count{job="proto"}`,
		`test_nhcb_histogram_count{job="om2"}`,
	}

	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			require.Eventually(t, func() bool {
				gcmVal, _, gcmErr := gcmAPI.Query(ctx, query, time.Now())
				if gcmErr != nil {
					return false
				}
				vec, ok := gcmVal.(model.Vector)
				if !ok || len(vec) == 0 {
					return false
				}
				return true
			}, 20*time.Minute, 2*time.Second, "metric %s should be present in GCM", query)
		})
	}
}
