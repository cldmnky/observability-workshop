package main

import (
	"context"
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
	mux.HandleFunc("/healthz", application.handleHealth)
	mux.HandleFunc("/", application.handleHome)
	mux.HandleFunc("/ping", application.handlePing)
	mux.HandleFunc("/error", application.handleError)
	mux.HandleFunc("/events", application.handleEvents)

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

	writeJSON(response, http.StatusOK, map[string]string{"message": "frontend is running", "service": application.serviceName})
}

func (application *frontendApp) handlePing(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	application.forward(response, request, "/api/ok")
}

func (application *frontendApp) handleError(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	application.forward(response, request, "/api/error")
}

func (application *frontendApp) handleEvents(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	application.forward(response, request, "/api/events")
}

func (application *frontendApp) forward(response http.ResponseWriter, request *http.Request, path string) {
	target := application.backendURL + path
	backendRequest, err := http.NewRequestWithContext(request.Context(), http.MethodGet, target, nil)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to build backend request")
		return
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

	contentType := backendResponse.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	response.Header().Set("Content-Type", contentType)
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
