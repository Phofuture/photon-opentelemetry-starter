package opentelCore

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Phofuture/photon-core-starter/utils/convert"

	runtimeMetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	metr "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
)

type Int64UpDownCounter struct {
	metr.Int64UpDownCounter
}

type Int64UpCounter struct {
	metr.Int64Counter
}

type Int64Guage struct {
	metr.Int64ObservableGauge
}

var (
	provider       *metric.MeterProvider
	upDownCounters = make(map[string]Int64UpDownCounter)
	upCounters     = make(map[string]Int64UpCounter)
	upDownMutex    sync.Mutex
	upMutex        sync.Mutex
)

// startMetricService 以 OTLP push 啟動 metric（PeriodicReader → 共用 collector）。
// v2.0.0 起移除 Prometheus 拉取式；metricPort / metricPath 已作廢，僅印 deprecation warning。
func startMetricService(ctx context.Context, config *Config) {
	if config.OpenTel.MetricPort != nil || config.OpenTel.MetricPath != nil {
		slog.Warn("[otel]openTel.metricPort / openTel.metricPath 已於 v2.0.0 作廢（Prometheus 拉取式移除），" +
			"metric 改走 OTLP push，這兩個設定不再有作用，請從 config 移除")
	}

	slog.Info("[otel]start metric service (OTLP push)")

	reader, err := newMetricReader(ctx)
	if err != nil {
		slog.Error("[otel]create metric exporter failed, metric disabled", "error", err)
		return
	}

	provider = metric.NewMeterProvider(
		metric.WithReader(reader),
		metric.WithResource(sharedRes),
	)
	otel.SetMeterProvider(provider)

	if err := runtimeMetrics.Start(); err != nil {
		slog.Error("[otel]start runtime metric error", "error", err)
	}
}

func newMetricReader(ctx context.Context) (metric.Reader, error) {
	var exp metric.Exporter
	var err error
	if collectorUseGRPC {
		exp, err = otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(collectorConn))
	} else {
		exp, err = otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(collectorEndpoint))
	}
	if err != nil {
		return nil, err
	}
	return metric.NewPeriodicReader(exp), nil
}

/*
Add meter up/down count
*/
func AddMeterUpDownCount(action, description, unit string, value int64, attributes ...attribute.KeyValue) {
	if provider == nil {
		return
	}
	upDownMutex.Lock()
	defer upDownMutex.Unlock()
	counter, ok := upDownCounters[action]
	if !ok {
		meter := provider.Meter(serviceName)
		c, err := meter.Int64UpDownCounter(
			action,
			metr.WithDescription(description),
			metr.WithUnit(unit),
		)
		if err != nil {
			slog.Error("[otel]add meter counter error", "error", err)
		}
		counter.Int64UpDownCounter = c
		upDownCounters[action] = counter
	}
	ctx := context.Background()
	counter.addMeterUpDownCount(ctx, value, metr.WithAttributes(attributes...))
}

func (c *Int64UpDownCounter) addMeterUpDownCount(ctx context.Context, value int64, options ...metr.AddOption) {
	c.Int64UpDownCounter.Add(ctx, value, options...)
}

func AddMeterCount(action, description, unit string, value uint64, attributes ...attribute.KeyValue) {
	if provider == nil {
		return
	}
	upMutex.Lock()
	defer upMutex.Unlock()
	counter, ok := upCounters[action]
	if !ok {
		meter := provider.Meter(serviceName)
		c, err := meter.Int64Counter(
			action,
			metr.WithDescription(description),
			metr.WithUnit(unit),
		)
		if err != nil {
			slog.Error("[otel]add meter counter error", "error", err)
		}
		counter.Int64Counter = c
		upCounters[action] = counter
	}
	ctx := context.Background()
	counter.addMeterCount(ctx, value, metr.WithAttributes(attributes...))
}

func (c *Int64UpCounter) addMeterCount(ctx context.Context, value uint64, options ...metr.AddOption) {
	c.Int64Counter.Add(ctx, convert.Signed[uint64, int64](value), options...)
}
