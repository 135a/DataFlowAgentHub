package otelsetup

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Init registers a TracerProvider with W3C TraceContext propagator.
// If HUB_OTEL_EXPORTER_ENDPOINT is set, enables OTLP gRPC trace export.
// Returns a shutdown function for cleanup on process exit.
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
