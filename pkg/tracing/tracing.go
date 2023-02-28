package tracing

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
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

	for key, val := range getDefaultSpanFields(ctx) {
		if !addedAttributes[key] {
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

func getDefaultSpanFields(ctx context.Context) map[string]string {
	defaultFields := make(map[string]string)

	b := baggage.FromContext(ctx)
	for _, member := range b.Members() {
		val, err := url.PathUnescape(member.Value())
		if err == nil {
			defaultFields[member.Key()] = val
		}
	}
	return defaultFields
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
	Host          string
	User          string
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
	tp := newTraceProvider(exp, opts)

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
	}
	return otlptracehttp.NewClient(opts...)
}

func newTraceProvider(exp *otlptrace.Exporter, opts TracerOpts) *sdktrace.TracerProvider {
	resource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("cli"),
	)
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource),
	)
}
