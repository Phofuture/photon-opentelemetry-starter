package opentelCore

import (
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
)

var (
	tp          *sdktrace.TracerProvider
	serviceName string

	// 三個 signal（trace / metric / log）共用的 collector 連線與 resource，於 Start 建立。
	collectorConn *grpc.ClientConn
	sharedRes     *resource.Resource
	// url 為 http(s):// 時無共用 grpc conn，各 exporter 走 http endpoint。
	collectorEndpoint string
	collectorUseGRPC  bool
)

type Protocol uint
