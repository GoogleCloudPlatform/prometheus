# google

This directory contains Google specific packages required by the GMP Prometheus fork and https://github.com/GoogleCloudPlatform/prometheus-engine

This code is meant to be deprecated eventually, once bits are upstreamed :

* `export`: In-memory export pipeline to Google Cloud Monitoring gRPC API. This
  Go package will be deprecated once Prometheus and GCM supports [Remote Write 2.0](https://prometheus.io/docs/specs/prw/remote_write_spec_2_0/) fully.
* `secrets`: Kubernetes secret provider implementation required for the scalable
  secure scrape support. This package will be deprecated once [PROM-47](https://github.com/prometheus/proposals/pull/47) is implemented.
* `lease`: Master-election feature for Prometheus. Not very commonly used feature, with the potential to upstream it -- no plans for upstream currently.
