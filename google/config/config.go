package config

// GoogleCloudConfig represents a custom Prometheus config entry
// required by the hot-reloading requirements in high security k8s setups.
//
// All configuration parameters overrides the corresponding EXTRA_ARGS environment
// or CLI flags.
type GoogleCloudConfig struct {
	Export GoogleCloudExportConfig `yaml:"export,omitempty"`
	Query  GoogleCloudQueryConfig  `yaml:"query,omitempty"`
}

// GoogleCloudExportConfig represents hot-reloadable ExporterOpts options.
type GoogleCloudExportConfig struct {
	// Compression controls the ExporterOpts.Compression setting.
	Compression string `yaml:"compression,omitempty"`
	// CredentialsFile controls the ExporterOpts.CredentialsFile setting.
	CredentialsFile string `yaml:"credentials,omitempty"`

	// EnableMatch allows additional control over matching filtering
	// given the go/gmp:matchstuck context.
	//
	// Available settings:
	// * not set: This config's Match settings are ignored. Custom export.match flags are used.
	// * false: Filtering feature is explicitly disabled; export matches all series.
	// * true: This config's Match settings are used. Overwrites all custom export.match flags.
	EnableMatch *bool `yaml:"enable_match,omitempty"`
	// Match, if EnableMatch is true, controls the ExporterOpts.Matchers setting.
	// It offers runtime change-able control of the "matchOneOf" filtering.
	// This filtering will skip certain series from entering the series cache (for export).
	// This data will still be ingested into Prometheus local storage.
	//
	// IMPORTANT: This option is prone to misconfiguration, thus opt-in and removed from the public docs.
	// See https://cloud.google.com/stackdriver/docs/managed-prometheus/setup-managed#filter-metrics.
	Match []string `yaml:"match,omitempty"`
}

// GoogleCloudQueryConfig represents hot-reloadable options for custom
// rule recording against GCM. This configuration is NOT used by Prometheus fork,
// but it's used for the GMP rule-evaluator needs, which uses Prometheus config as
// it's full configuration. More specifically, we host this option here due to
// historical decision to combine query and export fields together: https://github.com/GoogleCloudPlatform/prometheus-engine/commit/0dd3d48d79cc5b0c0209de22d9054e76473c6429#r163387087.
type GoogleCloudQueryConfig struct {
	ProjectID       string `yaml:"project_id,omitempty"`
	GeneratorURL    string `yaml:"generator_url,omitempty"`
	CredentialsFile string `yaml:"credentials,omitempty"`
}
