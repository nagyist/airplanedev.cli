package tracing

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

// Span is a wrapper around an OTEL span.
type Span struct {
	ctx      context.Context
	otelSpan trace.Span
}

// SetField sets an arbitrary key/value on a span.
func (s *Span) SetField(name string, value interface{}) *Span {
	s.otelSpan.SetAttributes(getAttribute(name, value))
	return s
}

// Finish terminates the span.
func (s *Span) Finish() {
	s.otelSpan.End()
}

// AddEvent adds an event into the span at the current time.
func (s *Span) AddEvent(name string) *Span {
	s.otelSpan.AddEvent(name)
	return s
}

// StartSpan creates a new span in the currently active trace.
func StartSpan(
	ctx context.Context,
	operation string,
	initialFields ...map[string]interface{},
) (context.Context, *Span) {
	return StartSpanWithTime(ctx, operation, time.Time{}, initialFields...)
}

// StartSpanWithTime creates a new span in the currently active trace using an arbitrary
// start time in the past.
func StartSpanWithTime(
	ctx context.Context,
	operation string,
	ts time.Time,
	initialFields ...map[string]interface{},
) (context.Context, *Span) {
	addedAttributes := make(map[string]bool)
	var attributes []attribute.KeyValue
	for _, fs := range initialFields {
		for key, val := range fs {
			addedAttributes[key] = true
			attributes = append(attributes, getAttribute(key, val))
		}
	}

	opts := []trace.SpanStartOption{
		trace.WithAttributes(attributes...),
		trace.WithTimestamp(ts),
	}

	ctx, otelSpan := otel.GetTracerProvider().Tracer("cli").
		Start(ctx, operation, opts...)
	return ctx, &Span{
		ctx,
		otelSpan,
	}
}

func getAttribute(key string, val interface{}) attribute.KeyValue {
	switch v := val.(type) {
	case string:
		return attribute.String(key, v)
	case bool:
		return attribute.Bool(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case []bool:
		return attribute.BoolSlice(key, v)
	case []string:
		return attribute.StringSlice(key, v)
	case []int:
		return attribute.IntSlice(key, v)
	case []int64:
		return attribute.Int64Slice(key, v)
	case []float64:
		return attribute.Float64Slice(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", val))
	}
}

// TracerOpts stores options for the tracing provider.
type TracerOpts struct {
	CollectorAddr string
	TeamID        string
	UserID        string
	UserEmail     string
	Version       string
}

// InitializeTracerProvider creates a new tracing provider for the CLI.
func InitializeTracerProvider(
	ctx context.Context,
	opts TracerOpts,
) (*sdktrace.TracerProvider, otlptrace.Client, error) {
	client := newTraceClient(ctx, opts.CollectorAddr)
	exp, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to initialize exporter")
	}

	// Create a new tracer provider with a batch span processor and the otlp exporter.
	tp, err := newTraceProvider(exp, opts)
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

func newTraceProvider(exp *otlptrace.Exporter, opts TracerOpts) (*sdktrace.TracerProvider, error) {
	// Automatically get lots of details about the user's environment.
	resources, err := resource.New(
		context.Background(),
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

// newFileExporter is an exporter than can be used instead of the http client one. For
// debugging purposes only.
func newFileExporter(w io.Writer) (sdktrace.SpanExporter, error) {
	return stdouttrace.New(
		stdouttrace.WithWriter(w),
		stdouttrace.WithPrettyPrint(),
		stdouttrace.WithoutTimestamps(),
	)
}
