// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promqle2etest

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	gcm "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/efficientgo/e2e"
	e2emon "github.com/efficientgo/e2e/monitoring"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/compliance/promqle2e"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var _ promqle2e.Backend = PrometheusRemoteWriteGCMBackend{}

// PrometheusRemoteWriteGCMBackend represents a vanilla Prometheus scraping
// metrics and pushing to GCM API via Prometheus Remote Write 2.0 protocol.
// This generally follows https://cloud.google.com/stackdriver/docs/managed-prometheus/setup-unmanaged.
type PrometheusRemoteWriteGCMBackend struct {
	Image string
	Name  string
	GCMSA []byte
}

func (p PrometheusRemoteWriteGCMBackend) Ref() string {
	return p.Name
}

// newPrometheusRemoteWrite creates a new Prometheus runnable configured with remote write 2.0 against GCM.
func newPrometheusRemoteWrite(env e2e.Environment, name string, image string, scrapeTargetAddress string, projectID string, location string, cluster string, gcmSA []byte) *e2emon.Prometheus {
	ports := map[string]int{"http": 9090}

	f := env.Runnable(name).WithPorts(ports).Future()
	credsFile := filepath.Join(f.Dir(), "gcm-sa.json")
	if err := os.WriteFile(credsFile, gcmSA, 0600); err != nil {
		return &e2emon.Prometheus{Runnable: e2e.NewFailedRunnable(name, fmt.Errorf("write JSON creds failed: %w", err))}
	}

	config := fmt.Sprintf(`
global:
  scrape_interval: 5s
  scrape_timeout: 5s
  convert_classic_histograms_to_nhcb: true
  external_labels:
    collector: %v
    project_id: %v
    location: %v
    cluster: %v
scrape_configs:
- job_name: 'test'
  scrape_interval: 5s
  scrape_timeout: 5s
  static_configs:
  - targets: [%s]
  metric_relabel_configs:
  - regex: instance
    action: labeldrop
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
  # write_relabel_configs:
  # - regex: '(__type__|__unit__)'
  #   action: labeldrop
`, name, projectID, location, cluster, scrapeTargetAddress, credsFile)
	if err := os.WriteFile(filepath.Join(f.Dir(), "prometheus.yml"), []byte(config), 0600); err != nil {
		return &e2emon.Prometheus{Runnable: e2e.NewFailedRunnable(name, fmt.Errorf("create prometheus config failed: %w", err))}
	}

	args := map[string]string{
		"--web.listen-address":                    fmt.Sprintf(":%d", ports["http"]),
		"--config.file":                           filepath.Join(f.Dir(), "prometheus.yml"),
		"--storage.tsdb.path":                     f.Dir(),
		"--enable-feature=exemplar-storage":       "",
		"--enable-feature=native-histograms":      "",
		"--enable-feature=promql-nhcb-as-classic": "",
		"--enable-feature=st-storage":             "",
		"--enable-feature=st-synthesis":           "",
		"--enable-feature=type-and-unit-labels":   "",
		"--enable-feature=xor2-encoding":          "",
		"--storage.tsdb.no-lockfile":              "",
		"--storage.tsdb.retention.time":           "1d",
		"--storage.tsdb.wal-compression":          "",
		"--storage.tsdb.min-block-duration":       "2h",
		"--storage.tsdb.max-block-duration":       "2h",
		"--web.enable-lifecycle":                  "",
		"--log.format":                            "json",
		"--log.level":                             "debug",
	}

	p := e2emon.AsInstrumented(f.Init(e2e.StartOptions{
		Image:     image,
		Command:   e2e.NewCommandWithoutEntrypoint("prometheus", e2e.BuildArgs(args)...),
		Readiness: e2e.NewHTTPReadinessProbe("http", "/-/ready", 200, 200),
		User:      strconv.Itoa(os.Getuid()),
	}), "http")

	return &e2emon.Prometheus{
		Runnable:     p,
		Instrumented: p,
	}
}

func (p PrometheusRemoteWriteGCMBackend) StartAndWaitReady(t testing.TB, env e2e.Environment) promqle2e.RunningBackend {
	t.Helper()

	ctx := t.Context()

	creds, err := google.CredentialsFromJSON(ctx, p.GCMSA, gcm.DefaultAuthScopes()...)
	if err != nil {
		t.Fatalf("create credentials from JSON: %s", err)
	}

	// Fake, does not matter.
	cluster := "pe-github-action"
	location := "europe-west3-a"

	cl, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("https://staging-monitoring.sandbox.googleapis.com/v1/projects/%s/location/global/prometheus", creds.ProjectID),
		Client:  oauth2.NewClient(ctx, creds.TokenSource),
	})
	if err != nil {
		t.Fatalf("create Prometheus client: %s", err)
	}

	replayer := promqle2e.StartIngestByScrapeReplayer(t, env)
	prom := newPrometheusRemoteWrite(env, p.Name, p.Image, replayer.Endpoint(env), creds.ProjectID, location, cluster, p.GCMSA)
	if err := e2e.StartAndWaitReady(prom); err != nil {
		t.Fatal(err)
	}

	return promqle2e.NewRunningScrapeReplayBasedBackend(
		replayer,
		map[string]string{
			"cluster":    cluster,
			"location":   location,
			"project_id": creds.ProjectID,
			"collector":  p.Name,
			"job":        "test",
		},
		v1.NewAPI(cl),
	)
}
