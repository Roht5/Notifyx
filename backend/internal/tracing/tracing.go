// Package tracing initializes a process-wide OpenTelemetry TracerProvider so spans
// created via otel.Tracer("notifyx") flow through the same export pipeline,
// regardless of which package created them (API handler, producer, consumer).
package tracing

import (
	"context"
	"io"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// tracerName is the single instrumentation scope used across the codebase — spans
// are distinguished by name/attributes, not by per-package tracer names, since this
// is a single service rather than a library with multiple consumers.
const tracerName = "notifyx"

// Init wires a TracerProvider that exports spans as JSON to w (e.g. a log file).
// There's no OTel collector in this deployment, so spans are written locally for
// local inspection / a future collector swap rather than shipped to a backend.
// Returns a shutdown func to flush+stop on graceful exit.
func Init(ctx context.Context, env string, w io.Writer) (func(context.Context) error, error) {
	exporter, err := stdouttrace.New(stdouttrace.WithWriter(w), stdouttrace.WithoutTimestamps())
	if err != nil {
		return nil, err
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("notifyx"),
		semconv.DeploymentEnvironment(env),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// Tracer returns the shared notifyx tracer. Call this rather than otel.Tracer
// directly so every span in the codebase shares one instrumentation scope name.
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}
