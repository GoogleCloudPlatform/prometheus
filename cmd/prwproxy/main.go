// Copyright 2025 Google LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command prwproxy is a minimal, stateful Prometheus Remote Write 2.0 sidecar
// proxy that adapts vanilla OSS Prometheus output to the stricter Google Cloud
// Monitoring (Monarch) data model:
//
//   - it synthesizes start timestamps for cumulative series that don't have one
//   - it splits untyped ("unknown") series into an explicit gauge stream and an
//     explicit cumulative counter stream
//
// See go/gmp:unknown-types (b/551762955).
//
// Example:
//
//	prwproxy \
//	  --web.listen-address=:9092 \
//	  --forward.url=https://monitoring.googleapis.com/v1/projects/$PROJECT_ID/location/global/prometheus/api/v1/write
//
// and point Prometheus at it:
//
//	remote_write:
//	  - url: http://localhost:9092/api/v1/write
//	    protobuf_message: io.prometheus.write.v2.Request
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	promslogflag "github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"

	"github.com/prometheus/prometheus/google/prwproxy"
	"github.com/prometheus/prometheus/storage/remote/googleiam"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "prwproxy:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		cfg            prwproxy.Config
		listenAddr     string
		writePath      string
		credentialsFil string
		useGoogleAuth  bool
		forwardTimeout time.Duration
		promslogConfig promslog.Config
	)

	a := kingpin.New("prwproxy", "A minimal Prometheus Remote Write 2.0 proxy that makes vanilla Prometheus output ingestible by Google Cloud Monitoring.")
	a.Version(version.Print("prwproxy"))
	a.HelpFlag.Short('h')

	a.Flag("web.listen-address", "Address to listen on for remote write requests, /metrics and /-/healthy.").
		Default(":9092").StringVar(&listenAddr)
	a.Flag("web.write-path", "HTTP path accepting remote write requests.").
		Default("/api/v1/write").StringVar(&writePath)

	a.Flag("forward.url", "PRW2 endpoint to forward the transformed requests to.").
		Required().StringVar(&cfg.ForwardURL)
	a.Flag("forward.timeout", "Timeout for a single forwarded request.").
		Default("30s").DurationVar(&forwardTimeout)
	a.Flag("forward.google-auth", "Attach Google Cloud credentials to forwarded requests.").
		Default("true").BoolVar(&useGoogleAuth)
	a.Flag("forward.credentials-file", "Google Cloud service account credentials file. Defaults to application default credentials.").
		StringVar(&credentialsFil)
	a.Flag("forward.max-body-size", "Maximum accepted compressed remote write body, in bytes.").
		Default(strconv.FormatInt(prwproxy.DefaultMaxBodySize, 10)).Int64Var(&cfg.MaxBodySize)

	a.Flag("unknown.handle", "Split untyped series into a gauge and a cumulative counter stream.").
		Default("true").BoolVar(&cfg.Transform.HandleUnknown)
	a.Flag("unknown.gauge-suffix", "Suffix appended to __name__ of the gauge stream of an untyped series. Empty leaves the name untouched.").
		Default(prwproxy.DefaultUnknownGaugeSuffix).StringVar(&cfg.Transform.UnknownGaugeSuffix)
	a.Flag("unknown.counter-suffix", "Suffix appended to __name__ of the cumulative counter stream of an untyped series. Empty leaves the name untouched.").
		Default(prwproxy.DefaultUnknownCounterSuffix).StringVar(&cfg.Transform.UnknownCounterSuffix)

	a.Flag("st.synthesize", "Synthesize start timestamps for cumulative series that arrive without one.").
		Default("true").BoolVar(&cfg.Transform.SynthesizeST)
	a.Flag("st.stale-after", "How long to keep start timestamp synthesis state for a series after its last sample.").
		Default(prwproxy.DefaultStaleAfter.String()).DurationVar(&cfg.Transform.StaleAfter)

	promslogflag.AddFlags(a, &promslogConfig)

	if _, err := a.Parse(os.Args[1:]); err != nil {
		return err
	}
	logger := promslog.New(&promslogConfig)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 100

	client := &http.Client{
		Transport: transport,
		Timeout:   forwardTimeout,
	}
	if useGoogleAuth {
		rt, err := googleiam.NewRoundTripper(&googleiam.Config{CredentialsFile: credentialsFil}, transport)
		if err != nil {
			return fmt.Errorf("setting up Google credentials: %w", err)
		}
		client.Transport = rt
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		versioncollector.NewCollector("prwproxy"),
	)

	proxy, err := prwproxy.New(cfg, client, logger, reg)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go proxy.Run(ctx)

	mux := http.NewServeMux()
	mux.Handle(writePath, proxy)
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	mux.HandleFunc("/-/healthy", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting prwproxy", "address", listenAddr, "write_path", writePath, "forward_url", cfg.ForwardURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return srv.Shutdown(shutdownCtx)
	}
}
