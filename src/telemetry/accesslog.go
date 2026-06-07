// Package telemetry provides shared observability setup for the workshop Go
// services. The accesslog file adds dual-instrumentation: Prometheus /metrics
// (scraped by the COO MonitoringStack via per-user ServiceMonitors) and stdout
// JSON access logs (shipped to LokiStack by cluster-logging).  OTel metrics and
// app-level logs continue to flow through the OTel SDK → sidecar collector →
// central collector pipeline and are UNCHANGED.
package telemetry

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ---------------------------------------------------------------------------
// Prometheus instrumentation – private registry, no global pollution
// ---------------------------------------------------------------------------

var (
	promReg = prometheus.NewRegistry()

	promHTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "prom_http_requests_total",
			Help: "Total HTTP requests handled (Prometheus client SDK).",
		},
		[]string{"method", "route", "status"},
	)

	promHTTPRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "prom_http_request_duration_seconds",
			Help:    "HTTP request latency histogram (Prometheus client SDK).",
			Buckets: prometheus.DefBuckets, // .005 – 10 seconds, OK for HTTP APIs
		},
		[]string{"method", "route"},
	)
)

func init() {
	promReg.MustRegister(promHTTPRequestsTotal)
	promReg.MustRegister(promHTTPRequestDurationSeconds)
}

// MetricsHandler returns an HTTP handler that exposes Prometheus metrics in
// text format on the private registry.  Register this at "/metrics".
func MetricsHandler() http.Handler {
	return promhttp.HandlerFor(promReg, promhttp.HandlerOpts{})
}

// ---------------------------------------------------------------------------
// Access log middleware – writes one JSON line per request to stdout.
// This bypasses the OTel log bridge so the two log streams are distinguishable
// in LokiStack: stdout JSON  ⇢  cluster-logging (Vector)  ⇢  LokiStack;
// OTel app logs  ⇢  sidecar collector  ⇢  central collector  ⇢  LokiStack.
// ---------------------------------------------------------------------------

// accessLog is a dedicated logger that always writes structured JSON to
// stdout, regardless of whether OTEL_ENABLED=true has re-routed slog.
var accessLog = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

// AccessLog is HTTP middleware that:
//  1. Writes a JSON access-log line to stdout after every request.
//  2. Records Prometheus counter + histogram labelled by method, route, status.
//
// It is designed to wrap an http.ServeMux so that request.Pattern is set
// before the middleware records the route label.
func AccessLog(serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		duration := time.Since(start)

		// Resolve route pattern – Go 1.22+ ServeMux sets r.Pattern after
		// matching.  Fall back to URL.Path if no pattern matched.
		route := r.Pattern
		if route == "" {
			route = r.URL.Path
		}
		statusStr := http.StatusText(rec.status)
		if statusStr == "" {
			statusStr = "UNKNOWN"
		}

		// --- stdout access log (one JSON line) ---
		accessLog.Info("access",
			"service", serviceName,
			"method", r.Method,
			"path", r.URL.Path,
			"route", route,
			"status", rec.status,
			"duration_ms", duration.Milliseconds(),
			"bytes", rec.written,
			"remote_addr", r.RemoteAddr,
		)

		// --- Prometheus metrics ---
		promHTTPRequestsTotal.WithLabelValues(r.Method, route, statusStr).Inc()
		promHTTPRequestDurationSeconds.WithLabelValues(r.Method, route).Observe(duration.Seconds())
	})
}

// statusRecorder wraps http.ResponseWriter to capture the response status code
// and the number of bytes written.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written int64
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	n, err := r.ResponseWriter.Write(p)
	r.written += int64(n)
	return n, err
}

// ---------------------------------------------------------------------------
// Compile-time check: ensure io.Writer is implemented (for the metrics
// handler's own unregistered-collector logging).
// ---------------------------------------------------------------------------
var _ io.Writer = (*statusRecorder)(nil)
