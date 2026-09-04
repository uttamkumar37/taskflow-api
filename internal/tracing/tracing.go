// Package tracing wires up OpenTelemetry distributed tracing. Every span
// created via otelhttp (the HTTP layer) and otelsql (the database layer)
// flows through the TracerProvider built here.
package tracing

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init builds and registers the global TracerProvider. When otlpEndpoint is
// empty, the provider is still installed (so every otelhttp/otelsql span
// creation call succeeds) but has no exporter attached, so spans are
// created and immediately dropped — effectively free, and behavior-neutral
// for anyone not running a tracing backend.
//
// The returned shutdown func flushes any pending spans and must be called
// before the process exits.
func Init(ctx context.Context, serviceName, version, otlpEndpoint string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}

	opts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}

	if otlpEndpoint != "" {
		// otlptracehttp.WithEndpointURL requires the *full* signal path
		// (unlike the general OTEL_EXPORTER_OTLP_ENDPOINT env var convention,
		// which is a base URL that collectors/SDKs auto-append /v1/traces
		// to) — passing the bare base URL here silently 404s against every
		// real OTLP/HTTP receiver (Jaeger, an OTel Collector, Tempo, ...).
		tracesURL := strings.TrimRight(otlpEndpoint, "/") + "/v1/traces"

		exporter, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpointURL(tracesURL),
		)
		if err != nil {
			return nil, fmt.Errorf("build otlp exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
