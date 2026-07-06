# PromQL e2e tests

This directory contains (manual for now) test suite that allows testing various Prometheus
compliance elements around different Prometheus metric cases going through various
pipelines to GCM.

## Usage

To run those tests you need:

1. Install Go and Docker if not on your machine.
2. Obtain GCM secret with the permissions to read and write to GCM for the test project of your choice. Put the JSON body into a `GCM_SECRET` envvar. 
3. Set `GMP_PROMETHEUS_IMAGE` envvar e.g. to locally build image, or `gke.gcr.io/prometheus-engine/prometheus:v2.53.4-gmp.0-gke.1`
3. Run make test from this directory.
