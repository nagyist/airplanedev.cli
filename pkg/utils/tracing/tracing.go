package tracing

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
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

	ctx, otelSpan := otel.GetTracerProvider().Tracer("github.com/airplanedev/cli").
		Start(ctx, operation, opts...)
	return ctx, &Span{
		ctx,
		otelSpan,
	}
}

// SetFieldOnTrace sets a field on the current span and every child of the current span.
// The value will be stringified.
func SetFieldOnTrace(ctx context.Context, name string, value interface{}) context.Context {
	return SetFieldsOnTrace(ctx, map[string]interface{}{name: value})
}

// SetFieldsOnTrace sets multiple fields on the current span and every child of the current span.
// The values will be stringified.
func SetFieldsOnTrace(ctx context.Context, fields map[string]interface{}) context.Context {
	b := baggage.FromContext(ctx)
	currentSpan := trace.SpanFromContext(ctx)

	for key, val := range fields {
		currentSpan.SetAttributes(getAttribute(key, val))

		str := fmt.Sprintf("%v", val)
		encodedStr := url.PathEscape(str)
		member, err := baggage.NewMember(key, encodedStr)
		if err != nil {
			log.Printf("error creating baggage member %q", err)
		} else {
			b, err = b.SetMember(member)
			if err != nil {
				log.Printf("error setting baggage member %q", err)
			}
		}
	}

	return baggage.ContextWithBaggage(ctx, b)
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
