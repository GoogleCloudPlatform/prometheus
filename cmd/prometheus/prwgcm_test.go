package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/util/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func TestRWtoGCM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	gcmSA := os.Getenv("GCM_SA_JSON")
	if gcmSA == "" {
		t.Skip("skipping as GCM_SA_JSON env var is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
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

	config := fmt.Sprintf(`
global:
  scrape_interval: 2s
  scrape_timeout: 2s
  convert_classic_histograms_to_nhcb: true
  external_labels:
    collector: %s
    project_id: %s
    location: %s
    cluster: %s
scrape_configs:
- job_name: 'test'
  scrape_interval: 2s
  scrape_timeout: 2s
  static_configs:
  - targets: ['127.0.0.1:%d']
  metric_relabel_configs:
  - regex: instance
    action: labeldrop
remote_write:
- name: "google_cloud"
  url: "https://staging-monitoring.sandbox.googleapis.com/v1/prometheus/api/v1/write"
  protobuf_message: "io.prometheus.write.v2.Request"
  send_exemplars: true
  queue_config:
    retry_on_http_429: true
    max_samples_per_send: 200
  google_iam:
    credentials_file: "%s"
`, collector, creds.ProjectID, location, cluster, port, credsFile)

	configFile := filepath.Join(tmpDir, "prometheus.yml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))

	prom := prometheusCommandWithLogging(
		t,
		configFile,
		port,
		"--storage.tsdb.path="+filepath.Join(tmpDir, "data"),
		"--enable-feature=exemplar-storage",
		"--enable-feature=native-histograms",
		"--enable-feature=st-storage",
		"--enable-feature=st-synthesis",
		"--enable-feature=type-and-unit-labels",
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

	// Wait for a few scrapes and flushes to GCM
	time.Sleep(15 * time.Second)

	// Queries
	localCl, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("http://127.0.0.1:%d", port),
	})
	require.NoError(t, err)
	localAPI := v1.NewAPI(localCl)

	gcmCl, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("https://staging-monitoring.sandbox.googleapis.com/v1/projects/%s/location/global/prometheus", creds.ProjectID),
		Client:  oauth2.NewClient(ctx, creds.TokenSource),
	})
	require.NoError(t, err)
	gcmAPI := v1.NewAPI(gcmCl)

	query := `go_goroutines{job="test"}`

	deadline := time.Now().Add(20 * time.Minute)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		localVal, warnings, err := localAPI.Query(ctx, query, time.Now())
		if err == nil {
			fmt.Printf("Local Warnings: %v\n", warnings)
			fmt.Printf("Local Result: %v\n", localVal)
		} else {
			fmt.Printf("Local Query Error: %v\n", err)
		}

		gcmVal, gcmWarnings, gcmErr := gcmAPI.Query(ctx, query, time.Now())
		if gcmErr == nil {
			fmt.Printf("GCM Warnings: %v\n", gcmWarnings)
			fmt.Printf("GCM Result: %v\n", gcmVal)
		} else {
			fmt.Printf("GCM Query Error: %v\n", gcmErr)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
}
