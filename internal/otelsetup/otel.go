package otelsetup

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Init 注册带有 W3C TraceContext 传播器的 TracerProvider。如果设置了 HUB_OTEL_EXPORTER_ENDPOINT，则启用 OTLP gRPC 追踪导出。返回用于进程退出时清理的关闭函数。
func Init() (func(context.Context) error, error) {
	opts := []sdktrace.TracerProviderOption{}

	if ep := os.Getenv("HUB_OTEL_EXPORTER_ENDPOINT"); ep != "" {
		exp, err := otlptracegrpc.New(context.Background(),
			otlptracegrpc.WithEndpoint(ep),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp.Shutdown, nil
}
