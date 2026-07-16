package opentelLog

import (
	"context"
	"log/slog"
	"runtime"
	"time"

	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

const scopeName = "github.com/Phofuture/photon-opentelemetry-starter"

// OpentelLogHandler 是一個把 slog.Record 轉成 OTel log record 的 slog.Handler。
// 關鍵：在 Handle() 當下才向 global 取 LoggerProvider，
// 若本 handler 早於 otel core init 建好，早期 log 會落到 noop，core init 後自動切到真正的 provider，
// 避開 Go sibling package init 順序陷阱（見 slot-game openfeign init-order 事件）。
type OpentelLogHandler struct {
	level slog.Level
	attrs []slog.Attr
}

func NewOpentelLogHandler(level slog.Level) *OpentelLogHandler {
	return &OpentelLogHandler{level: level}
}

func (h *OpentelLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *OpentelLogHandler) Handle(ctx context.Context, record slog.Record) error {
	logger := global.GetLoggerProvider().Logger(scopeName)

	var r otellog.Record
	r.SetTimestamp(record.Time)
	r.SetBody(otellog.StringValue(record.Message))
	r.SetSeverity(otelSeverity(record.Level))
	r.SetSeverityText(record.Level.String())

	for _, a := range h.attrs {
		r.AddAttributes(slogAttrToOtel(a))
	}
	record.Attrs(func(a slog.Attr) bool {
		r.AddAttributes(slogAttrToOtel(a))
		return true
	})

	// caller（取代舊 ERROR 分支那段會 panic 的 stack 字串串接）
	if record.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{record.PC})
		if f, _ := fs.Next(); f.File != "" {
			r.AddAttributes(
				otellog.String("code.filepath", f.File),
				otellog.Int("code.lineno", f.Line),
			)
		}
	}

	// 帶 ctx：OTel log SDK 會自動從 span context 取 trace_id / span_id 做關聯。
	logger.Emit(ctx, r)
	return nil
}

func (h *OpentelLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := *h
	nh.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &nh
}

func (h *OpentelLogHandler) WithGroup(_ string) slog.Handler {
	return h
}

func otelSeverity(level slog.Level) otellog.Severity {
	switch {
	case level >= slog.LevelError:
		return otellog.SeverityError
	case level >= slog.LevelWarn:
		return otellog.SeverityWarn
	case level >= slog.LevelInfo:
		return otellog.SeverityInfo
	default:
		return otellog.SeverityDebug
	}
}

func slogAttrToOtel(a slog.Attr) otellog.KeyValue {
	return otellog.KeyValue{Key: a.Key, Value: slogValueToOtel(a.Value)}
}

func slogValueToOtel(v slog.Value) otellog.Value {
	switch v.Kind() {
	case slog.KindString:
		return otellog.StringValue(v.String())
	case slog.KindInt64:
		return otellog.Int64Value(v.Int64())
	case slog.KindUint64:
		return otellog.Int64Value(int64(v.Uint64()))
	case slog.KindFloat64:
		return otellog.Float64Value(v.Float64())
	case slog.KindBool:
		return otellog.BoolValue(v.Bool())
	case slog.KindDuration:
		return otellog.Int64Value(int64(v.Duration()))
	case slog.KindTime:
		return otellog.StringValue(v.Time().Format(time.RFC3339Nano))
	default:
		return otellog.StringValue(v.String())
	}
}
