package opentelCore

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// buildResource 建一份 trace / metric / log 三個 signal 共用的 Resource。
// service.name 一律帶；version / environment 選填（未設則不帶）；instance.id 取 hostname。
func buildResource(ctx context.Context, config *Config) *resource.Resource {
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(config.OpenTel.ServiceName),
	}
	if config.OpenTel.ServiceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersionKey.String(config.OpenTel.ServiceVersion))
	}
	if config.OpenTel.Environment != "" {
		attrs = append(attrs, semconv.DeploymentEnvironmentKey.String(config.OpenTel.Environment))
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		attrs = append(attrs, semconv.ServiceInstanceIDKey.String(host))
	}

	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		// resource.New 只在 schema URL 衝突時報錯；退回只帶傳入屬性的 resource，不讓它擋住啟動。
		slog.Error("[otel]build resource error, fallback to attribute-only resource", "error", err)
		return resource.NewWithAttributes(semconv.SchemaURL, attrs...)
	}
	return res
}
