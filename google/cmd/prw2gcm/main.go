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
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/google/export/setup"
)

const (
	// NOTE: Similar to other Prometheus HTTP APIs, we assume it will be under the
	// https://monitoring.googleapis.com/v1/projects/PROJECT_ID/location/global/prometheus/api/v1 path.
	//
	// See: https://cloud.google.com/stackdriver/docs/managed-prometheus/query-api-ui#api-prometheus
	pathPrefix = "/v1/projects/"
	pathSuffix = "/location/global/prometheus/api/v1/write"
)

var (
	listenAddress = flag.String("listen-address", ":19091",
		"Address on which to expose metrics and the Remote Write 2.x handler.")
	logLevelFlag       = flag.String("log.level", "info", "Logging level; available values: 'debug', 'info', 'warn', 'error'.")
	forwardCredentials = flag.Bool("gcm.forward-credentials", false,
		"Enables mode where proxy will expect an HTTP 'Authorization' header with the Bearer token in the incoming Remote Write 2.x requests. "+
				"This token will be then forwarded to the gRPC GCM unary call. This mode allows running this proxy as a Prometheus sidecar, using Prometheus remote_write"+
				" auth setup. IMPORTANT: For this mode proxy, in the insecure environments, proxy should be served behind TLS.")
	credentialsFile = flag.String("gcm.credentials-file", "",
		"File with JSON-encoded credentials (service account or refresh token). Can be left empty if default credentials have sufficient permission.")
	gcmEndpoint = flag.String("gcm.endpoint", "monitoring.googleapis.com:443",
		"GCM API endpoint to send metric data to.")
	gcmUserAgentMode = flag.String("gcm.user-agent-mode", setup.UAModeUnspecified, fmt.Sprintf("Mode for user agent used for requests against the GCM API. Valid values are %q, %q, %q, %q or %q.", setup.UAModeGKE, setup.UAModeKubectl, setup.UAModeAVMW, setup.UAModeABM, setup.UAModeUnspecified))
)

// TODO: Add TLS options, recommended if this proxy should forward auth headers.
func main() {
	flag.Parse()

	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(*logLevelFlag)); err != nil {
		println("failed to parse -log.level flag", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	metrics := prometheus.NewRegistry()
	metrics.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	var g run.Group
	g.Add(run.SignalHandler(context.Background(), os.Interrupt, syscall.SIGTERM))
	{
		env := setup.UAEnvUnspecified
		// Default target fields if we can detect them in GCP.
		if metadata.OnGCE() {
			env = setup.UAEnvGCE
			cluster, _ := metadata.InstanceAttributeValue("cluster-name")
			if cluster != "" {
				env = setup.UAEnvGKE
			}
		}

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(metrics, promhttp.HandlerOpts{Registry: metrics, EnableOpenMetrics: true}))
		mux.HandleFunc("/-/healthy", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "prw2gcm is Healthy.\n")
		})
		mux.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "prw2gcm is Ready.\n")
		})
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		// Identity User Agent for all gRPC requests.
		ua := strings.TrimSpace(fmt.Sprintf("%s/%s %s (env:%s;mode:%s)",
			"prw2gcm", version.Version, "prw2-gcm", env, *gcmUserAgentMode))
		p, err := newProxy(
			context.Background(),
			proxyOpts{
				logger:             logger,
				credentialsFile:    *credentialsFile,
				forwardCredentials: *forwardCredentials,
				endpoint:           *gcmEndpoint,
				defaultUserAgent:   ua,
			},
		)
		if err != nil {
			logger.Error("failed to create proxy", "err", err)
			os.Exit(1)
		}

		mux.Handle(p.Handler(metrics))
		server := &http.Server{
			Handler: mux,
			Addr:    *listenAddress,
		}
		g.Add(func() error {
			logger.Info("starting web server for metrics", "listen", *listenAddress)
			return server.ListenAndServe()
		}, func(err error) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			_ = server.Shutdown(ctx)
		})
	}

	logger.Info("starting prw2gcm...")
	if err := g.Run(); err != nil {
		logger.Error("prw2gcm failed", "err", err)
		os.Exit(1)
	}
	logger.Info("prw2gcm finished")
}
