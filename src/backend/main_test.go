package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	otellog "go.opentelemetry.io/otel/log"
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

func findAttr(record sdklog.Record, key string) (otellog.Value, bool) {
	var (
		value otellog.Value
		found bool
	)
	record.WalkAttributes(func(attribute otellog.KeyValue) bool {
		if attribute.Key == key {
			value = attribute.Value
			found = true
			return false
		}
		return true
	})
	return value, found
}

func TestLoggingMiddlewareEmitsHttpRequestLog(t *testing.T) {
	previousProvider := otelglobal.GetLoggerProvider()
	previousLogger := slog.Default()
	defer otelglobal.SetLoggerProvider(previousProvider)
	defer slog.SetDefault(previousLogger)

	exporter := &testLogExporter{}
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)),
	)
	otelglobal.SetLoggerProvider(provider)
	slog.SetDefault(slog.New(otelslog.NewHandler("backend")))

	handler := loggingMiddleware("backend", http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusCreated)
	}))

	request := httptest.NewRequest(http.MethodPost, "/api/notes", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown logger provider: %v", err)
	}

	records := exporter.snapshot()
	if len(records) == 0 {
		t.Fatal("expected at least one emitted log record")
	}

	var httpRequestRecord *sdklog.Record
	for index := range records {
		record := records[index]
		if record.Body().AsString() == "http request" {
			httpRequestRecord = &record
			break
		}
	}

	if httpRequestRecord == nil {
		t.Fatalf("expected a log record with body %q", "http request")
	}

	method, ok := findAttr(*httpRequestRecord, "method")
	if !ok || method.AsString() != http.MethodPost {
		t.Fatalf("expected method attribute %q, got %q", http.MethodPost, method.AsString())
	}

	path, ok := findAttr(*httpRequestRecord, "path")
	if !ok || path.AsString() != "/api/notes" {
		t.Fatalf("expected path attribute %q, got %q", "/api/notes", path.AsString())
	}

	status, ok := findAttr(*httpRequestRecord, "status")
	if !ok || status.AsInt64() != int64(http.StatusCreated) {
		t.Fatalf("expected status attribute %d, got %d", http.StatusCreated, status.AsInt64())
	}
}
