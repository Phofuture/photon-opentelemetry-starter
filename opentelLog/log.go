package opentelLog

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

type OpentelLogger struct {
	logLevel slog.Level
	handler  slog.Handler
}

func NewOpentelLogger(logLevel slog.Level) *OpentelLogger {
	return &OpentelLogger{
		logLevel: logLevel,
		handler:  NewOpentelLogHandler(logLevel),
	}
}

// log 把一筆 log 送 OTLP（透過 handler → LoggerProvider），並保留 stdout（slog.Default）。
// 不再每筆 log 開一個 span，也不再把訊息塞 span 屬性。
func (l *OpentelLogger) log(ctx context.Context, level slog.Level, callerSkip int, msg string, args ...any) {
	if !l.handler.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	// +2：跳過 runtime.Callers 自己與本 frame，讓 caller 指向呼叫端。
	runtime.Callers(callerSkip+2, pcs[:])

	record := slog.NewRecord(time.Now(), level, msg, pcs[0])
	record.Add(args...)
	_ = l.handler.Handle(ctx, record)

	// 保留 stdout（k8s pod log / 本機除錯）
	slog.Default().Log(ctx, level, msg, args...)
}

func (l *OpentelLogger) Info(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelInfo, 2, msg, args...)
}

func (l *OpentelLogger) Error(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelError, 2, msg, args...)
}

func (l *OpentelLogger) Debug(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelDebug, 2, msg, args...)
}

func (l *OpentelLogger) Warn(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelWarn, 2, msg, args...)
}

func (l *OpentelLogger) DebugContext(msg string, args ...any) {
	l.Debug(context.Background(), msg, args...)
}

func (l *OpentelLogger) InfoContext(msg string, args ...any) {
	l.Info(context.Background(), msg, args...)
}

func (l *OpentelLogger) WarnContext(msg string, args ...any) {
	l.Warn(context.Background(), msg, args...)
}

func (l *OpentelLogger) ErrorContext(msg string, args ...any) {
	l.Error(context.Background(), msg, args...)
}
