---
title: prometheus
---

# prometheus

The Prometheus monitoring server



## Flags

| Flag | Description | Default |
| --- | --- | --- |
| <code class="text-nowrap">-h</code>, <code class="text-nowrap">--help</code> | Show context-sensitive help (also try --help-long and --help-man). |  |
| <code class="text-nowrap">--version</code> | Show application version. |  |
| <code class="text-nowrap">--config.file</code> | Prometheus configuration file path. | `prometheus.yml` |
| <code class="text-nowrap">--web.listen-address</code> | Address to listen on for UI, API, and telemetry. | `0.0.0.0:9090` |
| <code class="text-nowrap">--auto-gomemlimit.ratio</code> | The ratio of reserved GOMEMLIMIT memory to the detected maximum container or system memory | `0.9` |
| <code class="text-nowrap">--web.config.file</code> | [EXPERIMENTAL] Path to configuration file that can enable TLS or authentication. |  |
| <code class="text-nowrap">--web.read-timeout</code> | Maximum duration before timing out read of the request, and closing idle connections. | `5m` |
| <code class="text-nowrap">--web.max-connections</code> | Maximum number of simultaneous connections. | `512` |
| <code class="text-nowrap">--web.external-url</code> | The URL under which Prometheus is externally reachable (for example, if Prometheus is served via a reverse proxy). Used for generating relative and absolute links back to Prometheus itself. If the URL has a path portion, it will be used to prefix all HTTP endpoints served by Prometheus. If omitted, relevant URL components will be derived automatically. |  |
| <code class="text-nowrap">--web.route-prefix</code> | Prefix for the internal routes of web endpoints. Defaults to path of --web.external-url. |  |
| <code class="text-nowrap">--web.user-assets</code> | Path to static asset directory, available at /user. |  |
| <code class="text-nowrap">--web.enable-lifecycle</code> | Enable shutdown and reload via HTTP request. | `false` |
| <code class="text-nowrap">--web.enable-admin-api</code> | Enable API endpoints for admin control actions. | `false` |
| <code class="text-nowrap">--web.enable-remote-write-receiver</code> | Enable API endpoint accepting remote write requests. | `false` |
| <code class="text-nowrap">--web.console.templates</code> | Path to the console template directory, available at /consoles. | `consoles` |
| <code class="text-nowrap">--web.console.libraries</code> | Path to the console library directory. | `console_libraries` |
| <code class="text-nowrap">--web.page-title</code> | Document title of Prometheus instance. | `Prometheus Time Series Collection and Processing Server` |
| <code class="text-nowrap">--web.cors.origin</code> | Regex for CORS origin. It is fully anchored. Example: 'https?://(domain1|domain2)\.com' | `.*` |
| <code class="text-nowrap">--storage.tsdb.path</code> | Base path for metrics storage. Use with server mode only. | `data/` |
| <code class="text-nowrap">--storage.tsdb.retention</code> | [DEPRECATED] How long to retain samples in storage. This flag has been deprecated, use "storage.tsdb.retention.time" instead. Use with server mode only. |  |
| <code class="text-nowrap">--storage.tsdb.retention.time</code> | How long to retain samples in storage. When this flag is set it overrides "storage.tsdb.retention". If neither this flag nor "storage.tsdb.retention" nor "storage.tsdb.retention.size" is set, the retention time defaults to 15d. Units Supported: y, w, d, h, m, s, ms. Use with server mode only. |  |
| <code class="text-nowrap">--storage.tsdb.retention.size</code> | Maximum number of bytes that can be stored for blocks. A unit is required, supported units: B, KB, MB, GB, TB, PB, EB. Ex: "512MB". Based on powers-of-2, so 1KB is 1024B. Use with server mode only. |  |
| <code class="text-nowrap">--storage.tsdb.no-lockfile</code> | Do not create lockfile in data directory. Use with server mode only. | `false` |
| <code class="text-nowrap">--storage.tsdb.head-chunks-write-queue-size</code> | Size of the queue through which head chunks are written to the disk to be m-mapped, 0 disables the queue completely. Experimental. Use with server mode only. | `0` |
| <code class="text-nowrap">--storage.agent.path</code> | Base path for metrics storage. Use with agent mode only. | `data-agent/` |
| <code class="text-nowrap">--storage.agent.wal-compression</code> | Compress the agent WAL. Use with agent mode only. | `true` |
| <code class="text-nowrap">--storage.agent.retention.min-time</code> | Minimum age samples may be before being considered for deletion when the WAL is truncated Use with agent mode only. |  |
| <code class="text-nowrap">--storage.agent.retention.max-time</code> | Maximum age samples may be before being forcibly deleted when the WAL is truncated Use with agent mode only. |  |
| <code class="text-nowrap">--storage.agent.no-lockfile</code> | Do not create lockfile in data directory. Use with agent mode only. | `false` |
| <code class="text-nowrap">--storage.remote.flush-deadline</code> | How long to wait flushing sample on shutdown or config reload. | `1m` |
| <code class="text-nowrap">--storage.remote.read-sample-limit</code> | Maximum overall number of samples to return via the remote read interface, in a single query. 0 means no limit. This limit is ignored for streamed response types. Use with server mode only. | `5e7` |
| <code class="text-nowrap">--storage.remote.read-concurrent-limit</code> | Maximum number of concurrent remote read calls. 0 means no limit. Use with server mode only. | `10` |
| <code class="text-nowrap">--storage.remote.read-max-bytes-in-frame</code> | Maximum number of bytes in a single frame for streaming remote read response types before marshalling. Note that client might have limit on frame size as well. 1MB as recommended by protobuf by default. Use with server mode only. | `1048576` |
| <code class="text-nowrap">--rules.alert.for-outage-tolerance</code> | Max time to tolerate prometheus outage for restoring "for" state of alert. Use with server mode only. | `1h` |
| <code class="text-nowrap">--rules.alert.for-grace-period</code> | Minimum duration between alert and restored "for" state. This is maintained only for alerts with configured "for" time greater than grace period. Use with server mode only. | `10m` |
| <code class="text-nowrap">--rules.alert.resend-delay</code> | Minimum amount of time to wait before resending an alert to Alertmanager. Use with server mode only. | `1m` |
| <code class="text-nowrap">--rules.max-concurrent-evals</code> | Global concurrency limit for independent rules that can run concurrently. When set, "query.max-concurrency" may need to be adjusted accordingly. Use with server mode only. | `4` |
| <code class="text-nowrap">--alertmanager.notification-queue-capacity</code> | The capacity of the queue for pending Alertmanager notifications. Use with server mode only. | `10000` |
| <code class="text-nowrap">--query.lookback-delta</code> | The maximum lookback duration for retrieving metrics during expression evaluations and federation. Use with server mode only. | `5m` |
| <code class="text-nowrap">--query.timeout</code> | Maximum time a query may take before being aborted. Use with server mode only. | `2m` |
| <code class="text-nowrap">--query.max-concurrency</code> | Maximum number of queries executed concurrently. Use with server mode only. | `20` |
| <code class="text-nowrap">--query.max-samples</code> | Maximum number of samples a single query can load into memory. Note that queries will fail if they try to load more samples than this into memory, so this also limits the number of samples a query can return. Use with server mode only. | `50000000` |
| <code class="text-nowrap">--enable-feature</code> | Comma separated feature names to enable. Valid options: agent, auto-gomemlimit, exemplar-storage, expand-external-labels, memory-snapshot-on-shutdown, promql-per-step-stats, promql-experimental-functions, remote-write-receiver (DEPRECATED), extra-scrape-metrics, new-service-discovery-manager, auto-gomaxprocs, no-default-scrape-port, native-histograms, otlp-write-receiver, created-timestamp-zero-ingestion, concurrent-rule-eval. See https://prometheus.io/docs/prometheus/latest/feature_flags/ for more details. |  |
| <code class="text-nowrap">--log.level</code> | Only log messages with the given severity or above. One of: [debug, info, warn, error] | `info` |
| <code class="text-nowrap">--log.format</code> | Output format of log messages. One of: [logfmt, json] | `logfmt` |
| <code class="text-nowrap">--gmp.storage.delete-data-on-start</code> | [GMP fork experimental flag] If true, all the storage related data (e.g. blocks, lock file, WAL, head chunks) in the --storage.tsdb.path or --storage.agent.path (depending on the mode) will be deleted, right before opening the DB. As a result, all previously collected samples will be uncoverably dropped. Use it in setups where the availability is more important than the persistence between restarts, as replaying data can take time and resources. This flag is especially useful on Kubernetes with ephemeral storage (for consistency between pod vs container restart), remote write use cases that prioritize live data and when you want to auto-recover from the OOM crashloops without changing memory limits for Prometheus (see https://github.com/prometheus/prometheus/issues/13939). | `false` |
| <code class="text-nowrap">--export.disable</code> | Disable exporting to GCM. | `false` |
| <code class="text-nowrap">--export.endpoint</code> | GCM API endpoint to send metric data to. | `monitoring.googleapis.com:443` |
| <code class="text-nowrap">--export.compression</code> | The compression format to use for gRPC requests ('none' or 'gzip'). | `none` |
| <code class="text-nowrap">--export.credentials-file</code> | Credentials file for authentication with the GCM API. |  |
| <code class="text-nowrap">--export.label.project-id</code> | Default project ID set for all exported data. Prefer setting the external label "project_id" in the Prometheus configuration if not using the auto-discovered default. |  |
| <code class="text-nowrap">--export.user-agent-mode</code> | Mode for user agent used for requests against the GCM API. Valid values are "gke", "kubectl", "on-prem", "baremetal" or "unspecified". | `unspecified` |
| <code class="text-nowrap">--export.label.location</code> | The default location set for all exported data. Prefer setting the external label "location" in the Prometheus configuration if not using the auto-discovered default. |  |
| <code class="text-nowrap">--export.label.cluster</code> | The default cluster set for all scraped targets. Prefer setting the external label "cluster" in the Prometheus configuration if not using the auto-discovered default. |  |
| <code class="text-nowrap">--export.match</code> | A Prometheus time series matcher. Can be repeated. Every time series must match at least one of the matchers to be exported. This flag can be used equivalently to the match[] parameter of the Prometheus federation endpoint to selectively export data. (Example: --export.match='{job="prometheus"}' --export.match='{__name__=~"job:.*"}) |  |
| <code class="text-nowrap">--export.debug.metric-prefix</code> | Google Cloud Monitoring metric prefix to use. | `prometheus.googleapis.com` |
| <code class="text-nowrap">--export.debug.disable-auth</code> | Disable authentication (for debugging purposes). | `false` |
| <code class="text-nowrap">--export.debug.batch-size</code> | Maximum number of points to send in one batch to the GCM API. | `200` |
| <code class="text-nowrap">--export.debug.shard-count</code> | Number of shards that track series to send. | `1024` |
| <code class="text-nowrap">--export.debug.shard-buffer-size</code> | The buffer size for each individual shard. Each element in buffer (queue) consists of sample and hash. | `2048` |
| <code class="text-nowrap">--export.debug.fetch-metadata-timeout</code> | The total timeout for the initial gathering of the best-effort GCP data from the metadata server. This data is used for special labels required by Prometheus metrics (e.g. project id, location, cluster name), as well as information for the user agent. This is done on startup, so make sure this work to be faster than your readiness and liveliness probes. | `10s` |
| <code class="text-nowrap">--export.token-url</code> | The request URL to generate token that's needed to ingest metrics to the project |  |
| <code class="text-nowrap">--export.token-body</code> | The request Body to generate token that's needed to ingest metrics to the project. |  |
| <code class="text-nowrap">--export.quota-project</code> | The projectID of an alternative project for quota attribution. |  |
| <code class="text-nowrap">--export.ha.backend</code> | Which backend to use to coordinate HA pairs that both send metric data to the GCM API. Valid values are "none" or "kube" | `none` |
| <code class="text-nowrap">--export.ha.kube.config</code> | Path to kube config file. |  |
| <code class="text-nowrap">--export.ha.kube.namespace</code> | Namespace for the HA locking resource. Must be identical across replicas. May be set through the KUBE_NAMESPACE environment variable. |  |
| <code class="text-nowrap">--export.ha.kube.name</code> | Name for the HA locking resource. Must be identical across replicas. May be set through the KUBE_NAME environment variable. |  |


