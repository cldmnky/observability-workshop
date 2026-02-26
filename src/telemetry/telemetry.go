// Package telemetry sets up OpenTelemetry traces, metrics and logs.
// All telemetry is a no-op unless the OTEL_ENABLED environment variable
// is set to "true".
package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Enabled reports whether OTEL_ENABLED=true is set.
func Enabled() bool {
	return strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_ENABLED"))) == "true"
}

// Setup initialises the OpenTelemetry SDK when OTEL_ENABLED=true.
// When disabled it returns a no-op shutdown that callers can safely defer.
func Setup(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	if !Enabled() {
		return func(context.Context) error { return nil }, nil
	}

	endpoint := endpointFromEnv()

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}

	var shutdownFuncs []func(context.Context) error
	add := func(fn func(context.Context) error) {
		shutdownFuncs = append(shutdownFuncs, fn)
	}

	// --- Traces ---
	traceExp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)
	otel.SetTracerProvider(tp)
	add(tp.Shutdown)

	// --- Metrics ---
	metricExp, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(endpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(30*time.Second),
		)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)
	add(mp.Shutdown)

	// --- Logs ---
	logExp, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpointURL(endpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: log exporter: %w", err)
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	otelglobal.SetLoggerProvider(lp)
	add(lp.Shutdown)

	// --- Propagators (W3C TraceContext + Baggage) ---
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	shutdown = func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdownFuncs {
			if e := fn(ctx); e != nil {
				errs = append(errs, e)
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("telemetry shutdown: %v", errs)
		}
		return nil
	}
	return shutdown, nil
}

// endpointFromEnv returns the OTLP HTTP endpoint, defaulting to the sidecar.
func endpointFromEnv() string {
	ep := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if ep == "" {
		return "http://localhost:4318"
	}
	return ep
}
