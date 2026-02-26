package telemetry_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	otelglobal "go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

type testLogExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (exporter *testLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	for _, record := range records {
		exporter.records = append(exporter.records, record.Clone())
	}
	return nil
}

func (exporter *testLogExporter) Shutdown(context.Context) error { return nil }

func (exporter *testLogExporter) ForceFlush(context.Context) error { return nil }

func (exporter *testLogExporter) snapshot() []sdklog.Record {
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	result := make([]sdklog.Record, len(exporter.records))
	copy(result, exporter.records)
	return result
}

func newTestLoggerProvider() (*sdklog.LoggerProvider, *testLogExporter) {
	exporter := &testLogExporter{}
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)),
	)
	return provider, exporter
}

func totalRecords(exporter *testLogExporter) int {
	return len(exporter.snapshot())
}

func TestOtelslogBridgeEmitsRecords(t *testing.T) {
	provider, recorder := newTestLoggerProvider()
	otelglobal.SetLoggerProvider(provider)
	slog.SetDefault(slog.New(otelslog.NewHandler("test-service")))

	ctx := context.Background()
	slog.InfoContext(ctx, "test message", "key", "value")
	slog.WarnContext(ctx, "warn message", "code", 42)

	if err := provider.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown logger provider: %v", err)
	}

	if got := totalRecords(recorder); got < 2 {
		t.Fatalf("expected at least 2 log records, got %d", got)
	}
}

func TestHttpLoggingPathEmitsRecord(t *testing.T) {
	provider, recorder := newTestLoggerProvider()
	otelglobal.SetLoggerProvider(provider)
	slog.SetDefault(slog.New(otelslog.NewHandler("backend")))

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			start := time.Now()
			next.ServeHTTP(response, request)
			slog.InfoContext(
				request.Context(),
				"http request",
				"service", "backend",
				"method", request.Method,
				"path", request.URL.Path,
				"status", http.StatusOK,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}

	handler := middleware(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/ok", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown logger provider: %v", err)
	}

	found := false
	for _, record := range recorder.snapshot() {
		if record.Body().AsString() == "http request" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected a log record with body 'http request'")
	}
}
