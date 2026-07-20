package opentelCore

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Phofuture/photon-core-starter/utils/convert"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	runtimeMetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
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
	metricServer   *http.Server
	upDownCounters = make(map[string]Int64UpDownCounter)
	upCounters     = make(map[string]Int64UpCounter)
	upDownMutex    sync.Mutex
	upMutex        sync.Mutex
)

// startMetricService 啟動 metric，雙軌並存：
//   - OTLP push（PeriodicReader → 共用 collector，給 SigNoz）
//   - Prometheus 拉取式（SRE 監控用）：有設 metricPort 才啟動 /metrics HTTP server，
//     metricPath 未設時預設 /metrics
//
// 兩個 reader 掛在同一個 MeterProvider 上，資料一致。任一軌初始化失敗只停用該軌，不影響另一軌。
func startMetricService(ctx context.Context, config *Config) {
	var readers []metric.Option
	otlpEnabled := false

	// OTLP push reader
	if reader, err := newMetricReader(ctx); err != nil {
		slog.Error("[otel]create otlp metric exporter failed, otlp metric disabled", "error", err)
	} else {
		readers = append(readers, metric.WithReader(reader))
		otlpEnabled = true
	}

	// Prometheus 拉取式 reader
	if config.OpenTel.MetricPort != nil {
		if promReader, err := promexporter.New(); err != nil {
			slog.Error("[otel]create prometheus exporter failed, prometheus metric disabled", "error", err)
		} else {
			readers = append(readers, metric.WithReader(promReader))
			startPrometheusServer(config)
		}
	}

	if len(readers) == 0 {
		slog.Error("[otel]no metric reader available, metric disabled")
		return
	}

	slog.Info("[otel]start metric service", "otlpPush", otlpEnabled, "prometheusPull", metricServer != nil)

	provider = metric.NewMeterProvider(
		append(readers, metric.WithResource(sharedRes))...,
	)
	otel.SetMeterProvider(provider)

	if err := runtimeMetrics.Start(); err != nil {
		slog.Error("[otel]start runtime metric error", "error", err)
	}
}

// startPrometheusServer 啟動 Prometheus 拉取端點（與 v1.x 相同走 default registry，
// 保留 go_* / process_* 等既有 exposition，SRE 舊 dashboard 不受影響）。
func startPrometheusServer(config *Config) {
	path := "/metrics"
	if config.OpenTel.MetricPath != nil {
		path = *config.OpenTel.MetricPath
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())
	metricServer = &http.Server{
		Addr:              ":" + *config.OpenTel.MetricPort,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	slog.Info("[otel]prometheus metric listening", "port", *config.OpenTel.MetricPort, "path", path)
	go func() {
		if err := metricServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("[otel]start prometheus metric http service error", "error", err)
		}
	}()
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
