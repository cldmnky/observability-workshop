package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

type frontendApp struct {
	client      *http.Client
	backendURL  string
	serviceName string
}

func main() {
	addr := envOrDefault("FRONTEND_ADDR", ":8080")
	backendURL := strings.TrimRight(envOrDefault("BACKEND_URL", "http://backend:8081"), "/")
	serviceName := envOrDefault("SERVICE_NAME", "frontend")

	application := &frontendApp{
		client:      &http.Client{Timeout: 10 * time.Second},
		backendURL:  backendURL,
		serviceName: serviceName,
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("/healthz", application.handleHealth)
	mux.HandleFunc("/", application.handleHome)
	mux.HandleFunc("/ping", application.handlePing)
	mux.HandleFunc("/error", application.handleError)
	mux.HandleFunc("/events", application.handleEvents)
	mux.HandleFunc("/api/notes/export.md", application.handleNotesExport)
	mux.HandleFunc("/api/notes", application.handleNotes)
	mux.HandleFunc("/api/notes/", application.handleNoteByID)

	server := &http.Server{
		Addr:              addr,
		Handler:           loggingMiddleware(serviceName, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("service=%s msg=starting addr=%s backend_url=%s", serviceName, addr, backendURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("service=%s msg=listen_failed err=%v", serviceName, err)
		}
	}()

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-signalChannel

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("service=%s msg=shutdown_failed err=%v", serviceName, err)
	}
	log.Printf("service=%s msg=shutdown_complete", serviceName)
}

func (application *frontendApp) handleHealth(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "service": application.serviceName})
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

		log.Printf(
			"service=%s method=%s path=%s status=%d duration_ms=%d remote_addr=%s",
			serviceName,
			request.Method,
			request.URL.Path,
			recorder.status,
			duration.Milliseconds(),
			request.RemoteAddr,
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
