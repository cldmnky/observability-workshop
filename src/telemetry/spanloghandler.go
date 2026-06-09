// Package telemetry provides shared observability setup for the workshop Go
// services.  spanloghandler wraps the OTel slog bridge to append trace and
// span IDs to every log message that carries active span context.
package telemetry

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/trace"
)

// ---------------------------------------------------------------------------
// Custom slog.Handler that enriches log messages with trace / span IDs.
// ---------------------------------------------------------------------------

// spanLogHandler wraps an otelslog.Handler and appends trace and span IDs
// to every log record that has an active span in its context.  Logs that
// are created without a context (e.g. startup slog.Info calls) pass through
// unchanged.
//
// This gives every OTel-correlated log line a visible [trace:xxx span:yyy]
// suffix that users can copy into the Traces view for instant log-to-trace
// correlation.
type spanLogHandler struct {
	next slog.Handler
}

func (h *spanLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle extracts the span from ctx and appends trace/span IDs to the log
// message before forwarding to the wrapped OTel handler.
func (h *spanLogHandler) Handle(ctx context.Context, record slog.Record) error {
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		sc := span.SpanContext()
		record.Message = fmt.Sprintf("%s [trace:%s span:%s]",
			record.Message,
			sc.TraceID().String(),
			sc.SpanID().String(),
		)
	}
	return h.next.Handle(ctx, record)
}

func (h *spanLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &spanLogHandler{next: h.next.WithAttrs(attrs)}
}

func (h *spanLogHandler) WithGroup(name string) slog.Handler {
	return &spanLogHandler{next: h.next.WithGroup(name)}
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

// NewSpanLogHandler returns a slog.Handler that enriches OTel-correlated
// log messages with trace and span IDs before they are sent to the OTel
// log pipeline.
//
// Usage in main.go:
//
//	if telemetry.Enabled() {
//	    slog.SetDefault(slog.New(telemetry.NewSpanLogHandler(serviceName)))
//	}
func NewSpanLogHandler(serviceName string) slog.Handler {
	return &spanLogHandler{next: otelslog.NewHandler(serviceName)}
}
