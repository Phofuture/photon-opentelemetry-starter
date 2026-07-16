package opentelCore

import (
	"context"
	"log/slog"
)

func Shutdown(ctx context.Context) error {
	slog.Info("[otel]shutdown otel")

	// flush trace
	if tp != nil {
		if err := tp.Shutdown(ctx); err != nil {
			slog.Error("[otel]shutdown trace provider error", "error", err)
		}
	}
	// flush metric（OTLP push 需 flush 尾端 metric）
	if provider != nil {
		if err := provider.Shutdown(ctx); err != nil {
			slog.Error("[otel]shutdown meter provider error", "error", err)
		}
	}
	// flush log
	shutdownLoggerProvider(ctx)

	// 關閉共用 collector 連線
	if collectorConn != nil {
		if err := collectorConn.Close(); err != nil {
			slog.Error("[otel]close collector conn error", "error", err)
		}
	}
	return nil
}
