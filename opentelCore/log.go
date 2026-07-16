package opentelCore

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	otellogglobal "go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

var loggerProvider *sdklog.LoggerProvider

// startLogExporter 建 LoggerProvider（OTLP push，走共用 collector + resource），並設為 OTel global。
// 由 Start() 在 conn / resource 就緒後呼叫；opentelLog 的 handler 惰性向 global 取用，避開 init 順序。
func startLogExporter(ctx context.Context) error {
	var exp sdklog.Exporter
	var err error
	if collectorUseGRPC {
		exp, err = otlploggrpc.New(ctx, otlploggrpc.WithGRPCConn(collectorConn))
	} else {
		exp, err = otlploghttp.New(ctx, otlploghttp.WithEndpointURL(collectorEndpoint))
	}
	if err != nil {
		slog.Error("[otel]create log exporter failed, log OTLP disabled", "error", err)
		return err
	}

	loggerProvider = sdklog.NewLoggerProvider(
		sdklog.WithResource(sharedRes),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
	)
	otellogglobal.SetLoggerProvider(loggerProvider)
	slog.Info("[otel]start log service (OTLP push)")
	return nil
}

func shutdownLoggerProvider(ctx context.Context) {
	if loggerProvider != nil {
		if err := loggerProvider.Shutdown(ctx); err != nil {
			slog.Error("[otel]shutdown logger provider error", "error", err)
		}
	}
}
