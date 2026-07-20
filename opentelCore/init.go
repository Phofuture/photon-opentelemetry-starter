package opentelCore

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Phofuture/photon-core-starter/configuration"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 初始化 opentelmetery
func Start(startContext context.Context) (err error) {
	slog.Info("init otel")

	ctx, cancel := context.WithTimeout(startContext, time.Second*5)
	defer cancel()

	config, err := configuration.Get[Config](ctx)
	if err != nil {
		return
	}
	serviceName = config.OpenTel.ServiceName

	// 建共用 resource（trace / metric / log 三 signal 共用）
	sharedRes = buildResource(ctx, &config)

	// 建共用 collector 連線：grpc:// → 一條共用 grpc conn；否則走 http endpoint
	if err = dialCollector(ctx, &config); err != nil {
		return
	}

	// 初始化 trace exporter
	if err = initTraceExporter(ctx, &config); err != nil {
		return
	}

	// 啟動 metric service（OTLP push）
	startMetricService(ctx, &config)

	// 啟動 log exporter（OTLP push）
	if err = startLogExporter(ctx); err != nil {
		return
	}

	return
}

// dialCollector 依 openTel.url 解析出共用連線。grpc:// 時建立一條 *grpc.ClientConn 給三個 signal 共用。
func dialCollector(ctx context.Context, config *Config) error {
	if strings.HasPrefix(config.OpenTel.URL, "grpc://") {
		host := config.OpenTel.URL[len("grpc://"):]
		conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			slog.Error("[otel]dial to otel grpc server failed", "error", err, "host", host)
			return err
		}
		collectorConn = conn
		collectorUseGRPC = true
		collectorEndpoint = host
		return nil
	}
	collectorUseGRPC = false
	collectorEndpoint = config.OpenTel.URL
	return nil
}

func initTraceExporter(ctx context.Context, config *Config) error {
	var traceExporter *otlptrace.Exporter
	var err error
	if collectorUseGRPC {
		traceExporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(collectorConn))
	} else {
		traceExporter, err = otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(collectorEndpoint))
	}
	if err != nil {
		slog.Error("[otel]create trace exporter failed", "error", err, "url", config.OpenTel.URL)
		return err
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tp = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(buildSampler(config)),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(sharedRes),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return nil
}

// buildSampler 依 openTel.sampleRatio 決定取樣策略，一律 ParentBased（父 span 有決策則沿用）。
// 未設：維持全採（相容舊行為，需明確設 ratio 才開始取樣）；>=1 全採；<=0 不採自身、僅跟隨父。
func buildSampler(config *Config) sdktrace.Sampler {
	ratio := config.OpenTel.SampleRatio
	if ratio == nil {
		return sdktrace.AlwaysSample()
	}
	switch {
	case *ratio >= 1:
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case *ratio <= 0:
		return sdktrace.ParentBased(sdktrace.NeverSample())
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(*ratio))
	}
}
