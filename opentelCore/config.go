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

		// Deprecated：Prometheus 拉取式已於 v2.0.0 移除，改走 OTLP push。
		// 欄位保留僅為相容舊 yaml，不再有任何作用；若有設值啟動會印 deprecation warning。
		MetricPort *string `yaml:"metricPort"`
		MetricPath *string `yaml:"metricPath"`
	} `yaml:"openTel"`
}
