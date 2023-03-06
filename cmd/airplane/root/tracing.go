package root

import (
	"context"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// tracerOpts stores options for the tracing provider.
type tracerOpts struct {
	CollectorAddr string
	TeamID        string
	UserID        string
	UserEmail     string
	Version       string
}

// initializeTracerProvider creates a new tracing provider for the CLI.
func initializeTracerProvider(
	ctx context.Context,
	opts tracerOpts,
) (*sdktrace.TracerProvider, otlptrace.Client, error) {
	client := newTraceClient(ctx, opts.CollectorAddr)
	exp, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to initialize exporter")
	}

	// Create a new tracer provider with a batch span processor and the otlp exporter.
	tp, err := newTraceProvider(ctx, exp, opts)
	if err != nil {
		return nil, nil, err
	}

	// Set the Tracer Provider and the W3C Trace Context propagator as globals
	otel.SetTracerProvider(tp)

	// Register the trace context and baggage propagators so data is propagated
	// across services/processes.
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return tp, client, nil
}

func newTraceClient(ctx context.Context, collectorAddr string) otlptrace.Client {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(collectorAddr),
		otlptracehttp.WithURLPath("/v1/traces"),
	}
	return otlptracehttp.NewClient(opts...)
}

func newTraceProvider(
	ctx context.Context,
	exp *otlptrace.Exporter,
	opts tracerOpts,
) (*sdktrace.TracerProvider, error) {
	// Automatically get lots of details about the user's environment.
	resources, err := resource.New(
		ctx,
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("cli"),
			attribute.String("authn_team_id", opts.TeamID),
			attribute.String("authn_user_id", opts.UserID),
			attribute.String("authn_user_email", opts.UserEmail),
			attribute.String("cli_version", opts.Version),
		),
	)
	if err != nil {
		return nil, errors.Wrap(err, "creating new resource")
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resources),
	), nil
}
