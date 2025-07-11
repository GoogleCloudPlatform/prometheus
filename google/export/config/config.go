package config

// GoogleCloudConfig represents a custom Prometheus config entry
// required by the hot-reloading requirements in high security k8s setups.
type GoogleCloudConfig struct {
	Export GoogleCloudExportConfig `yaml:"export,omitempty"`
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
