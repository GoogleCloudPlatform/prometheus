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

//go:build gcme2e
// +build gcme2e

package ingeste2etest

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"testing"

	gcm "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/efficientgo/core/testutil"
	"github.com/efficientgo/e2e"
	e2einteractive "github.com/efficientgo/e2e/interactive"
	e2emon "github.com/efficientgo/e2e/monitoring"
	"golang.org/x/oauth2/google"
)

// gcmServiceAccountOrFail gets the Google SA JSON content from GCM_SECRET
// environment variable or fails.
func gcmServiceAccountOrFail(t testing.TB) []byte {
	saJSON := []byte(os.Getenv("GCM_SECRET"))
	if len(saJSON) == 0 {
		t.Fatal("gcmServiceAccountOrFail: no GCM_SECRET env var provided, can't run the test")
	}
	return saJSON
}

// PrometheusForkGCMBackend represents a Prometheus GMP fork scraping
// metrics and pushing to GCM API for consumption.
type PrometheusForkGCMBackend struct {
	Image string
	Name  string
	GCMSA []byte
}

func (p PrometheusForkGCMBackend) Ref() string {
	return p.Name
}

// newPrometheus creates a new Prometheus runnable.
func newPrometheus(env e2e.Environment, name string, image string, targetAddrs map[string]string, flagOverride func(dir string) map[string]string) *e2emon.Prometheus {
	ports := map[string]int{"http": 9090}

	f := env.Runnable(name).WithPorts(ports).Future()

	var staticConfigs string
	// To keep it deterministic, sort the instance names.
	var instances []string
	for instance := range targetAddrs {
		instances = append(instances, instance)
	}
	sort.Strings(instances)

	for _, instance := range instances {
		addr := targetAddrs[instance]
		staticConfigs += fmt.Sprintf(`
  - targets: [%q]
    labels:
      instance: %q`, addr, instance)
	}

	config := fmt.Sprintf(`
global:
  external_labels:
    collector: %v
scrape_configs:
- job_name: 'test'
  scrape_interval: 5s
  scrape_timeout: 5s
  static_configs:%s
`, name, staticConfigs)
	if err := os.WriteFile(filepath.Join(f.Dir(), "prometheus.yml"), []byte(config), 0600); err != nil {
		return &e2emon.Prometheus{Runnable: e2e.NewFailedRunnable(name, fmt.Errorf("create prometheus config failed: %w", err))}
	}

	args := map[string]string{
		"--web.listen-address":               fmt.Sprintf(":%d", ports["http"]),
		"--config.file":                      filepath.Join(f.Dir(), "prometheus.yml"),
		"--storage.tsdb.path":                f.Dir(),
		"--enable-feature=exemplar-storage":  "",
		"--enable-feature=native-histograms": "",
		"--storage.tsdb.no-lockfile":         "",
		"--storage.tsdb.retention.time":      "1d",
		"--storage.tsdb.wal-compression":     "",
		"--storage.tsdb.min-block-duration":  "2h",
		"--storage.tsdb.max-block-duration":  "2h",
		"--web.enable-lifecycle":             "",
		"--log.format":                       "json",
		"--log.level":                        "debug",
	}
	if flagOverride != nil {
		args = e2e.MergeFlagsWithoutRemovingEmpty(args, flagOverride(f.Dir()))
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

func (p PrometheusForkGCMBackend) StartAndWaitReady(t testing.TB, env e2e.Environment, targetAddrs map[string]string) *e2emon.Prometheus {
	t.Helper()

	ctx := t.Context()

	creds, err := google.CredentialsFromJSON(ctx, p.GCMSA, gcm.DefaultAuthScopes()...)
	if err != nil {
		t.Fatalf("create credentials from JSON: %s", err)
	}

	// Fake, does not matter.
	cluster := "pe-github-action"
	location := "europe-west3-a"

	prom := newPrometheus(env, p.Name, p.Image, targetAddrs, func(dir string) map[string]string {
		if err := os.WriteFile(filepath.Join(dir, "gcm-sa.json"), p.GCMSA, 0600); err != nil {
			t.Fatalf("write JSON creds: %s", err)
		}

		// Flags as per https://cloud.google.com/stackdriver/docs/managed-prometheus/setup-unmanaged#gmp-binary.
		return map[string]string{
			"--export.label.project-id": creds.ProjectID,
			"--export.label.location":   location,
			"--export.label.cluster":    cluster,
			"--export.credentials-file": filepath.Join(dir, "gcm-sa.json"),
		}
	})
	if err := e2e.StartAndWaitReady(prom); err != nil {
		t.Fatal(err)
	}

	return prom
}

// Reproducing interleaved metrics.
func generateKongMetrics(scrapeNum int) string {
	var buf bytes.Buffer
	buf.WriteString("# HELP kong_kong_latency_ms Latency added by Kong and enabled plugins for each service/route in Kong\n")
	buf.WriteString("# TYPE kong_kong_latency_ms histogram\n")

	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"add_bucket\",le=\"10\"} %d\n", scrapeNum*2)
	if scrapeNum >= 3 {
		fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"add_bucket\",le=\"20\"} %d\n", scrapeNum*5)
	}
	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"add_bucket\",le=\"50\"} %d\n", scrapeNum*8)
	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"add_bucket\",le=\"100\"} %d\n", scrapeNum*10)
	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"add_bucket\",le=\"+Inf\"} %d\n", scrapeNum*10)
	mDec := scrapeNum
	if scrapeNum >= 3 {
		mDec = scrapeNum - 2
	}
	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-missing\",le=\"10\"} %d\n", scrapeNum*12)
	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-missing\",le=\"20\"} %d\n", scrapeNum*15)
	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-missing\",le=\"50\"} %d\n", scrapeNum*18)
	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-missing\",le=\"100\"} %d\n", scrapeNum*110)
	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-missing\",le=\"+Inf\"} %d\n", scrapeNum*110)
	fmt.Fprintf(&buf, "kong_kong_latency_ms_count{case=\"add_bucket\"} %d\n", scrapeNum*10)
	if scrapeNum < 6 {
		fmt.Fprintf(&buf, "kong_kong_latency_ms_count{case=\"counter-missing\"} %d\n", mDec*110)
	}
	fmt.Fprintf(&buf, "kong_kong_latency_ms_sum{case=\"add_bucket\"} %d\n", scrapeNum*310)
	fmt.Fprintf(&buf, "kong_kong_latency_ms_sum{case=\"counter-missing\"} %d\n", scrapeNum*1310)
	return buf.String()
}

//func generateKongMetrics(scrapeNum int) string {
//	var buf bytes.Buffer
//	buf.WriteString("# HELP kong_kong_latency_ms Latency added by Kong and enabled plugins for each service/route in Kong\n")
//	buf.WriteString("# TYPE kong_kong_latency_ms histogram\n")
//
//	//// a. healthy histogram has 5 buckets sum and count and slowly increases every scrape
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"healthy\",le=\"10\"} %d\n", scrapeNum*2)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"healthy\",le=\"20\"} %d\n", scrapeNum*5)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"healthy\",le=\"50\"} %d\n", scrapeNum*8)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"healthy\",le=\"100\"} %d\n", scrapeNum*10)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"healthy\",le=\"+Inf\"} %d\n", scrapeNum*10)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_sum{case=\"healthy\"} %d\n", scrapeNum*310)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_count{case=\"healthy\"} %d\n", scrapeNum*10)
//
//	// b. add_bucket is slowly increasing. On 3rd scrape new bucket arrives
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"add_bucket\",le=\"10\"} %d\n", scrapeNum*2)
//	if scrapeNum >= 3 {
//		fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"add_bucket\",le=\"20\"} %d\n", scrapeNum*5)
//	}
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"add_bucket\",le=\"50\"} %d\n", scrapeNum*8)
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"add_bucket\",le=\"100\"} %d\n", scrapeNum*10)
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"add_bucket\",le=\"+Inf\"} %d\n", scrapeNum*10)
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_sum{case=\"add_bucket\"} %d\n", scrapeNum*310)
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_count{case=\"add_bucket\"} %d\n", scrapeNum*10)
//
//	// c. counter-missing is slowly increasing. On 3rd scrape counter is decreasing value. On 4rd scrape going up. On 5rd scrape removed.
//	mDec := scrapeNum
//	if scrapeNum >= 3 {
//		mDec = scrapeNum - 2
//	}
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-missing\",le=\"10\"} %d\n", scrapeNum*2)
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-missing\",le=\"20\"} %d\n", scrapeNum*5)
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-missing\",le=\"50\"} %d\n", scrapeNum*8)
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-missing\",le=\"100\"} %d\n", scrapeNum*10)
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-missing\",le=\"+Inf\"} %d\n", scrapeNum*10)
//	fmt.Fprintf(&buf, "kong_kong_latency_ms_sum{case=\"counter-missing\"} %d\n", scrapeNum*310)
//	if scrapeNum < 6 {
//		fmt.Fprintf(&buf, "kong_kong_latency_ms_count{case=\"counter-missing\"} %d\n", mDec*10)
//	}
//	//
//	//// d. counter-decreasing is slowly increasing. On 3rd scrape counter is decreasing value, then slowly increasing
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-decreasing\",le=\"10\"} %d\n", scrapeNum*2)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-decreasing\",le=\"20\"} %d\n", scrapeNum*5)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-decreasing\",le=\"50\"} %d\n", scrapeNum*8)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-decreasing\",le=\"100\"} %d\n", scrapeNum*10)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_bucket{case=\"counter-decreasing\",le=\"+Inf\"} %d\n", scrapeNum*10)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_sum{case=\"counter-decreasing\"} %d\n", scrapeNum*310)
//	//fmt.Fprintf(&buf, "kong_kong_latency_ms_count{case=\"counter-decreasing\"} %d\n", mDec*10)
//
//	return buf.String()
//}

func TestIngestE2E_ProxyKong(t *testing.T) {
	env, err := e2e.NewDockerEnvironment("ingest-e2e")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(env.Close)

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	var mu sync.Mutex
	var scrapeCount int

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" && r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		scrapeCount++
		currentScrape := scrapeCount
		mu.Unlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(generateKongMetrics(currentScrape)))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", handler)
	mux.HandleFunc("/", handler)

	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(l)
	}()
	t.Cleanup(func() {
		_ = srv.Close()
	})

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	targetAddrs := map[string]string{
		"proxy-kong": net.JoinHostPort(env.HostAddr(), port),
	}

	backend := PrometheusForkGCMBackend{
		Name:  "gmp-prom-ingest",
		Image: "us-east1-docker.pkg.dev/gpe-test-1/public/prometheus-engine:v2.53.5-gmp.5-dev1", //"gcr.io/gke-release/prometheus-engine/prometheus:v2.45.3-gmp.18-gke.2", // "gcr.io/gke-release/prometheus-engine/prometheus@sha256:68038912eddca33347de1e85ffae8b785832303875957bd58d579a52ce8b6f93",
		GCMSA: gcmServiceAccountOrFail(t),
	}

	prom := backend.StartAndWaitReady(t, env, targetAddrs)
	t.Logf("Started Prometheus fork successfully: %s", prom.Name())

	mu.Lock()
	scrapeCount = 0
	mu.Unlock()

	testutil.Ok(t, e2einteractive.OpenInBrowser("http://"+prom.Endpoint("http")))
	testutil.Ok(t, e2einteractive.RunUntilEndpointHit())
}
