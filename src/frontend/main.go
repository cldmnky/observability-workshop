package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/cldmnky/observability-workshop/src/telemetry"
)

//go:embed static/*
var staticFiles embed.FS

// codeFiles embeds the source files copied into code/ by the Containerfile.
// During local development the directory is sparse (only .gitkeep) and the
// /api/code endpoints return an empty listing – this is expected.
//
//go:embed all:code
var codeFiles embed.FS

type frontendApp struct {
	client      *http.Client
	backendURL  string
	serviceName string
}

func main() {
	addr := envOrDefault("FRONTEND_ADDR", ":8080")
	backendURL := strings.TrimRight(envOrDefault("BACKEND_URL", "http://backend:8081"), "/")
	serviceName := envOrDefault("SERVICE_NAME", "frontend")

	// ------------------------------------------------------------------
	// Telemetry – set up traces, metrics and logs when OTEL_ENABLED=true
	// ------------------------------------------------------------------
	ctx := context.Background()
	telShutdown, err := telemetry.Setup(ctx, serviceName)
	if err != nil {
		slog.Error("telemetry setup failed", "service", serviceName, "err", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = telShutdown(shutCtx)
	}()
	// When OTEL is active route all structured log output via the SDK so
	// that log records are correlated with the active trace.
	if telemetry.Enabled() {
		slog.SetDefault(slog.New(otelslog.NewHandler(serviceName)))
	}

	// HTTP client – wrap transport with otelhttp so outgoing requests
	// carry W3C trace-context and are recorded as child spans.
	clientTransport := http.DefaultTransport
	if telemetry.Enabled() {
		clientTransport = otelhttp.NewTransport(http.DefaultTransport)
	}
	application := &frontendApp{
		client:      &http.Client{Timeout: 10 * time.Second, Transport: clientTransport},
		backendURL:  backendURL,
		serviceName: serviceName,
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("/healthz", application.handleHealth)
	mux.HandleFunc("/api/code", application.handleCodeList)
	mux.HandleFunc("/api/code/", application.handleCodeFile)
	mux.HandleFunc("/", application.handleHome)
	mux.HandleFunc("/ping", application.handlePing)
	mux.HandleFunc("/error", application.handleError)
	mux.HandleFunc("/events", application.handleEvents)
	mux.HandleFunc("/api/notes/export.md", application.handleNotesExport)
	mux.HandleFunc("/api/notes", application.handleNotes)
	mux.HandleFunc("/api/notes/", application.handleNoteByID)

	// otelhttp.NewHandler is the outermost layer: it extracts the incoming
	// traceparent header, creates a server span, and enriches the request
	// context before control passes inward.  loggingMiddleware sits *inside*
	// otelhttp so that slog.InfoContext receives a context that already holds
	// the active span – enabling trace_id / span_id in every log record.
	var handler http.Handler = loggingMiddleware(serviceName, mux)
	if telemetry.Enabled() {
		handler = otelhttp.NewHandler(handler, serviceName,
			// Name spans "METHOD /path" so they are readable in Tempo.
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return r.Method + " " + r.URL.Path
			}),
		)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("starting", "service", serviceName, "addr", addr, "backend_url", backendURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen failed", "service", serviceName, "err", err)
			os.Exit(1)
		}
	}()

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-signalChannel

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		slog.Error("shutdown failed", "service", serviceName, "err", err)
	}
	slog.Info("shutdown complete", "service", serviceName)
}

func (application *frontendApp) handleHealth(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "service": application.serviceName})
}

// handleCodeList returns a JSON array of all embedded source file paths,
// relative to the code/ root (e.g. ["backend/main.go", "go.mod", ...]).
func (application *frontendApp) handleCodeList(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var paths []string
	_ = fs.WalkDir(codeFiles, "code", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// Strip the "code/" prefix so callers use clean relative paths.
		rel := strings.TrimPrefix(path, "code/")
		if rel == ".gitkeep" || rel == "" {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})

	if paths == nil {
		paths = []string{}
	}
	sort.Strings(paths)
	writeJSON(response, http.StatusOK, map[string]any{"files": paths})
}

// handleCodeFile returns the raw content of a single embedded source file.
// The request path must be /api/code/<relative-path>, e.g. /api/code/backend/main.go.
func (application *frontendApp) handleCodeFile(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rel := strings.TrimPrefix(request.URL.Path, "/api/code/")
	if rel == "" || strings.Contains(rel, "..") {
		writeError(response, http.StatusBadRequest, "invalid path")
		return
	}

	content, err := codeFiles.ReadFile("code/" + rel)
	if err != nil {
		writeError(response, http.StatusNotFound, "file not found")
		return
	}

	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(content)
}

func (application *frontendApp) handleHome(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		writeError(response, http.StatusNotFound, "not found")
		return
	}
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	content, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to load frontend")
		return
	}

	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(content)
}

func (application *frontendApp) handlePing(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	application.forwardGet(response, request, "/api/ok")
}

func (application *frontendApp) handleError(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	application.forwardGet(response, request, "/api/error")
}

func (application *frontendApp) handleEvents(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	application.forwardGet(response, request, "/api/events")
}

func (application *frontendApp) handleNotes(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	application.forwardWithRequestMethod(response, request, "/api/notes")
}

func (application *frontendApp) handleNoteByID(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPut && request.Method != http.MethodDelete {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identifier := strings.TrimPrefix(request.URL.Path, "/api/notes/")
	if identifier == "" || strings.Contains(identifier, "/") {
		writeError(response, http.StatusBadRequest, "invalid note id")
		return
	}
	application.forwardWithRequestMethod(response, request, "/api/notes/"+identifier)
}

func (application *frontendApp) handleNotesExport(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	application.forwardGet(response, request, "/api/notes/export.md")
}

func (application *frontendApp) forwardGet(response http.ResponseWriter, request *http.Request, path string) {
	application.proxyToBackend(response, request, http.MethodGet, path)
}

func (application *frontendApp) forwardWithRequestMethod(response http.ResponseWriter, request *http.Request, path string) {
	application.proxyToBackend(response, request, request.Method, path)
}

func (application *frontendApp) proxyToBackend(response http.ResponseWriter, request *http.Request, method string, path string) {
	target := application.backendURL + path
	var requestBody []byte

	if request.Body != nil {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			writeError(response, http.StatusBadRequest, "failed to read request body")
			return
		}
		requestBody = body
	}

	backendRequest, err := http.NewRequestWithContext(request.Context(), method, target, bytes.NewReader(requestBody))
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to build backend request")
		return
	}

	contentType := request.Header.Get("Content-Type")
	if contentType != "" {
		backendRequest.Header.Set("Content-Type", contentType)
	}

	backendResponse, err := application.client.Do(backendRequest)
	if err != nil {
		writeError(response, http.StatusBadGateway, "backend unavailable")
		return
	}
	defer backendResponse.Body.Close()

	body, err := io.ReadAll(backendResponse.Body)
	if err != nil {
		writeError(response, http.StatusBadGateway, "failed reading backend response")
		return
	}

	if downstreamType := backendResponse.Header.Get("Content-Type"); downstreamType != "" {
		response.Header().Set("Content-Type", downstreamType)
	}
	if disposition := backendResponse.Header.Get("Content-Disposition"); disposition != "" {
		response.Header().Set("Content-Disposition", disposition)
	}

	response.WriteHeader(backendResponse.StatusCode)
	_, _ = response.Write(body)
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(statusCode int) {
	recorder.status = statusCode
	recorder.ResponseWriter.WriteHeader(statusCode)
}

func loggingMiddleware(serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: response, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		duration := time.Since(start)
		// Use slog with the request context so that – when OTEL is active –
		// log records are automatically correlated with the active trace span.
		slog.InfoContext(
			request.Context(),
			"http request",
			"service", serviceName,
			"method", request.Method,
			"path", request.URL.Path,
			"status", recorder.status,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", request.RemoteAddr,
		)
	})
}

func writeError(response http.ResponseWriter, statusCode int, message string) {
	writeJSON(response, statusCode, map[string]string{"error": message})
}

func writeJSON(response http.ResponseWriter, statusCode int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(statusCode)
	_ = json.NewEncoder(response).Encode(payload)
}
