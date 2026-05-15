package otelsetup

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Init registers a TracerProvider (MVP：无 exporter，便于后续接 OTLP）与 W3C TraceContext 传播器。
// 返回值用于进程退出时释放资源。
func Init() (func(context.Context) error, error) {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp.Shutdown, nil
}
