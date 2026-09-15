# prwproxy (prototype)

A minimal, stateful **Prometheus Remote Write 2.0 proxy** meant to run as a
sidecar next to a *vanilla* OSS Prometheus, making its output ingestible by the
Google Cloud Monitoring (GCM) PRW2 endpoint.

```
prometheus --(PRW2)--> prwproxy --(PRW2)--> GCM PRW2 endpoint
```

Design: go/gmp:unknown-types, tracked in b/551762955. This is **Phase 1** of
that plan (short-term sidecar), which unblocks the GMP unfork for the ~1.5% of
collection traffic that carries untyped metrics.

## Why

Monarch (the GCM backend) is strongly typed and requires a start timestamp (ST)
for cumulative series. Vanilla Prometheus emits neither for:

1. **Untyped ("unknown") metrics** — legacy text-format exporters (MySQL,
   RabbitMQ, ...) and recording rules. The PRW2 endpoint rejects them.
2. **Cumulative series without an ST** — e.g. targets that don't expose
   OpenMetrics `_created` and are scraped by a Prometheus that isn't doing ST
   synthesis itself.

The GMP Prometheus fork solves both inside the binary today. This proxy moves
that logic out of the fork.

## What it does

### 1. Start timestamp synthesis

For `COUNTER` and `HISTOGRAM` series whose samples arrive with
`start_timestamp == 0`, the proxy keeps per-series state and:

- records the first observation as the reference point (that sample is
  **dropped**, it only anchors the series),
- re-bases every later value against the reference (`value - starting`) and
  stamps it with the synthesized ST,
- on a counter reset, restarts from a fresh ST of `sample_timestamp - 1ms`.

This is not a reimplementation: it calls the exact same code the Prometheus
scrape loop uses for `--enable-feature=...` ST synthesis, which was extracted
into [`model/stsynthesis`](../../model/stsynthesis) for this purpose.

### 2. Unknown type splitting

An untyped series is split into two explicitly typed streams, mirroring what the
GMP fork's exporter does (`google/export/series_cache.go`) and what the
equivalent OpenTelemetry Collector recipe does:

```yaml
transform/unknown-counter:
  metric_statements:
    - context: metric
      statements:
        - copy_metric(Concat([metric.name, "unknowncounter"], ":")) where metric.metadata["prometheus.type"] == "unknown" and not HasSuffix(metric.name, ":unknowncounter")
        - convert_gauge_to_sum("cumulative", true) where HasSuffix(metric.name, ":unknowncounter")
        - set(metric.name, Substring(metric.name, 0, Len(metric.name)-Len(":unknowncounter"))) where HasSuffix(metric.name, ":unknowncounter")
```

| stream  | `__name__`                 | PRW2 type | value                |
| ------- | -------------------------- | --------- | -------------------- |
| gauge   | `<name>/unknown`           | `GAUGE`   | raw, no ST           |
| counter | `<name>/unknown:counter`   | `COUNTER` | re-based, with an ST |

> [!NOTE]
> The suffixes exist so users migrating off the fork keep querying the **same**
> GCM metric types (`prometheus.googleapis.com/<name>/unknown` and
> `.../unknown:counter`). If the receiving endpoint derives the suffix from the
> PRW2 metric type itself, set `--unknown.gauge-suffix=""` and
> `--unknown.counter-suffix=""`; both streams then keep the original name and
> only differ by type, exactly like the OTel recipe above.

## Usage

```bash
prwproxy \
  --web.listen-address=:9092 \
  --forward.url=https://monitoring.googleapis.com/v1/projects/$PROJECT_ID/location/global/prometheus/api/v1/write
```

```yaml
# prometheus.yml
remote_write:
  - url: http://localhost:9092/api/v1/write
    protobuf_message: io.prometheus.write.v2.Request
```

Downstream status codes and the PRW2 `X-Prometheus-Remote-Write-*-Written`
headers are passed straight back, so Prometheus' remote write queue keeps its
normal retry and backoff behaviour.

`/metrics` exposes `prwproxy_*` metrics (tracked series, split series,
synthesized STs, dropped samples, request outcomes and latency).

## Resource profile

State is one small struct per cumulative/untyped series (~150 B/series
including map overhead), garbage collected `--st.stale-after` (default 10m)
after the last sample of a series. CPU overhead is a hash map lookup and a
subtraction per sample.

## Prototype caveats

- Series state is keyed by the labels hash, so a hash collision makes two series
  share synthesis state. Fine for a prototype, must be fixed before production.
- State is in-memory only: a restart re-anchors every series, losing one sample
  per series and resetting STs.
- Untyped **native histograms** are forwarded untouched (they are not a thing in
  practice).
- PRW1 is rejected — it carries neither metadata nor start timestamps, so there
  is nothing useful to do with it.
