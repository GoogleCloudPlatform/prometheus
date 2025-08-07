package config

// GoogleCloudConfig represents a custom Prometheus config entry
// required by the hot-reloading requirements in high security k8s setups.
type GoogleCloudConfig struct {
	Export GoogleCloudExportConfig `yaml:"export,omitempty"`
	Query  GoogleCloudQueryConfig  `yaml:"query,omitempty"`
}

// GoogleCloudExportConfig represents hot-reloadable ExporterOpts options.
type GoogleCloudExportConfig struct {
	// Compression controls the ExporterOpts.Compression setting.
	Compression string `yaml:"compression,omitempty"`
	// CredentialsFile controls ExporterOpts.CredentialsFile
	CredentialsFile string `yaml:"credentials,omitempty"`

	// Deprecated: This option no longer works, see https://github.com/GoogleCloudPlatform/prometheus-engine/pull/1688
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
