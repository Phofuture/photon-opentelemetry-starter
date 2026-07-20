package opentelCore

type Config struct {
	OpenTel struct {
		URL         string `yaml:"url"`
		ServiceName string `yaml:"serviceName"`

		// Resource 維度（皆低基數、選填；未設時該屬性不帶）
		Environment    string `yaml:"environment"`    // deployment.environment：dev / uat / prod
		ServiceVersion string `yaml:"serviceVersion"` // service.version：git commit / 版號

		// Trace 取樣率（0.0~1.0，未設 / <=0 時預設全採；階段 3 生效）
		SampleRatio *float64 `yaml:"sampleRatio"`

		// Prometheus 拉取式（SRE 監控用，與 OTLP push 並存）：
		// 有設 metricPort 才啟動 /metrics HTTP server；metricPath 未設時預設 /metrics。
		MetricPort *string `yaml:"metricPort"`
		MetricPath *string `yaml:"metricPath"`
	} `yaml:"openTel"`
}
