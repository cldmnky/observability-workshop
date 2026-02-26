package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/cldmnky/observability-workshop/src/telemetry"
)

type backendApp struct {
	client      *http.Client
	databaseURL string
	notifierURL string
	serviceName string
}

type databaseEventRequest struct {
	Source  string `json:"source"`
	Method  string `json:"method"`
	Route   string `json:"route"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func main() {
	addr := envOrDefault("BACKEND_ADDR", ":8081")
	databaseURL := strings.TrimRight(envOrDefault("DATABASE_API_URL", "http://database:8082"), "/")
	notifierURL := strings.TrimRight(envOrDefault("NOTIFIER_URL", "http://notifier:8083"), "/")
	serviceName := envOrDefault("SERVICE_NAME", "backend")

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
	if telemetry.Enabled() {
		slog.SetDefault(slog.New(otelslog.NewHandler(serviceName)))
	}

	// HTTP client – otelhttp transport propagates trace context downstream.
	clientTransport := http.DefaultTransport
	if telemetry.Enabled() {
		clientTransport = otelhttp.NewTransport(http.DefaultTransport)
	}
	application := &backendApp{
		client:      &http.Client{Timeout: 10 * time.Second, Transport: clientTransport},
		databaseURL: databaseURL,
		notifierURL: notifierURL,
		serviceName: serviceName,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", application.handleHealth)
	mux.HandleFunc("/api/ok", application.handleOK)
	mux.HandleFunc("/api/error", application.handleError)
	mux.HandleFunc("/api/events", application.handleEvents)
	mux.HandleFunc("/api/notes/export.md", application.handleNotesExport)
	mux.HandleFunc("/api/notes", application.handleNotes)
	mux.HandleFunc("/api/notes/", application.handleNoteByID)

	// otelhttp outermost so the span-enriched context flows into loggingMiddleware.
	var handler http.Handler = loggingMiddleware(serviceName, mux)
	if telemetry.Enabled() {
		handler = otelhttp.NewHandler(handler, serviceName,
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
		slog.Info("starting", "service", serviceName, "addr", addr, "database_api_url", databaseURL)
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

func (application *backendApp) handleHealth(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "service": application.serviceName})
}

func (application *backendApp) handleOK(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	err := application.createDatabaseEvent(request.Context(), databaseEventRequest{
		Source:  application.serviceName,
		Method:  request.Method,
		Route:   request.URL.Path,
		Status:  http.StatusOK,
		Message: "successful request",
	})
	if err != nil {
		writeError(response, http.StatusBadGateway, "failed to store event")
		return
	}

	writeJSON(response, http.StatusOK, map[string]string{"result": "ok", "service": application.serviceName})
}

func (application *backendApp) handleError(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	err := application.createDatabaseEvent(request.Context(), databaseEventRequest{
		Source:  application.serviceName,
		Method:  request.Method,
		Route:   request.URL.Path,
		Status:  http.StatusNotFound,
		Message: "simulated error response",
	})
	if err != nil {
		writeError(response, http.StatusBadGateway, "failed to store event")
		return
	}

	writeJSON(response, http.StatusNotFound, map[string]string{"error": "simulated error", "service": application.serviceName})
}

func (application *backendApp) handleEvents(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	targetURL := fmt.Sprintf("%s/events?limit=100", application.databaseURL)
	databaseRequest, err := http.NewRequestWithContext(request.Context(), http.MethodGet, targetURL, nil)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to build request")
		return
	}

	databaseResponse, err := application.client.Do(databaseRequest)
	if err != nil {
		writeError(response, http.StatusBadGateway, "database service unavailable")
		return
	}
	defer databaseResponse.Body.Close()

	body, err := io.ReadAll(databaseResponse.Body)
	if err != nil {
		writeError(response, http.StatusBadGateway, "failed reading database response")
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(databaseResponse.StatusCode)
	_, _ = response.Write(body)
}

func (application *backendApp) handleNotes(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// For note creation, capture the request body so we can forward both to
	// the database (via proxyDatabase) and to the notifier service.
	var title string
	if request.Method == http.MethodPost && request.Body != nil {
		body, err := io.ReadAll(request.Body)
		if err == nil {
			request.Body = io.NopCloser(bytes.NewReader(body))
			var nr struct {
				Title string `json:"title"`
			}
			_ = json.Unmarshal(body, &nr)
			title = nr.Title
		}
	}

	application.proxyDatabase(response, request, "/notes")

	// Notify the notifier service after a successful create. The call uses the
	// active request context so the outgoing HTTP span is linked to the current
	// trace. Errors are intentionally ignored – notification is best-effort.
	if request.Method == http.MethodPost {
		_ = application.callNotifier(request.Context(), "created", title)
	}
}

func (application *backendApp) handleNoteByID(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPut && request.Method != http.MethodDelete {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identifier := strings.TrimPrefix(request.URL.Path, "/api/notes/")
	if identifier == "" || strings.Contains(identifier, "/") {
		writeError(response, http.StatusBadRequest, "invalid note id")
		return
	}
	application.proxyDatabase(response, request, "/notes/"+identifier)

	// Send notification for mutating operations. Best-effort, errors ignored.
	switch request.Method {
	case http.MethodPut:
		_ = application.callNotifier(request.Context(), "updated", "")
	case http.MethodDelete:
		_ = application.callNotifier(request.Context(), "deleted", "")
	}
}

func (application *backendApp) handleNotesExport(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	application.proxyDatabase(response, request, "/notes/export.md")
}

func (application *backendApp) proxyDatabase(response http.ResponseWriter, request *http.Request, path string) {
	targetURL := application.databaseURL + path
	var bodyBuffer []byte

	if request.Body != nil {
		readBody, err := io.ReadAll(request.Body)
		if err != nil {
			writeError(response, http.StatusBadRequest, "failed reading request body")
			return
		}
		bodyBuffer = readBody
	}

	databaseRequest, err := http.NewRequestWithContext(request.Context(), request.Method, targetURL, bytes.NewReader(bodyBuffer))
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to build request")
		return
	}

	contentType := request.Header.Get("Content-Type")
	if contentType != "" {
		databaseRequest.Header.Set("Content-Type", contentType)
	}

	databaseResponse, err := application.client.Do(databaseRequest)
	if err != nil {
		writeError(response, http.StatusBadGateway, "database service unavailable")
		return
	}
	defer databaseResponse.Body.Close()

	responseBody, err := io.ReadAll(databaseResponse.Body)
	if err != nil {
		writeError(response, http.StatusBadGateway, "failed reading database response")
		return
	}

	if downstreamType := databaseResponse.Header.Get("Content-Type"); downstreamType != "" {
		response.Header().Set("Content-Type", downstreamType)
	}
	if disposition := databaseResponse.Header.Get("Content-Disposition"); disposition != "" {
		response.Header().Set("Content-Disposition", disposition)
	}

	response.WriteHeader(databaseResponse.StatusCode)
	_, _ = response.Write(responseBody)
}

// callNotifier sends a note lifecycle event to the notifier service.
// ctx is the active request context so W3C trace context headers are forwarded,
// linking the outgoing span to the current trace.
func (application *backendApp) callNotifier(ctx context.Context, action, title string) error {
	if application.notifierURL == "" {
		return nil
	}
	payload, err := json.Marshal(map[string]string{"action": action, "title": title})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		application.notifierURL+"/notify", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := application.client.Do(req)
	if err != nil {
		slog.WarnContext(ctx, "notifier unavailable", "err", err)
		return nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// createDatabaseEvent posts an event to the database service.
// ctx is threaded through so that outgoing HTTP calls carry the active span.
func (application *backendApp) createDatabaseEvent(ctx context.Context, payload databaseEventRequest) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/events", application.databaseURL), bytes.NewReader(jsonPayload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := application.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("database returned status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
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
