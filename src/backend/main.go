package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type backendApp struct {
	client      *http.Client
	databaseURL string
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
	serviceName := envOrDefault("SERVICE_NAME", "backend")

	application := &backendApp{
		client:      &http.Client{Timeout: 10 * time.Second},
		databaseURL: databaseURL,
		serviceName: serviceName,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", application.handleHealth)
	mux.HandleFunc("/api/ok", application.handleOK)
	mux.HandleFunc("/api/error", application.handleError)
	mux.HandleFunc("/api/events", application.handleEvents)

	server := &http.Server{
		Addr:              addr,
		Handler:           loggingMiddleware(serviceName, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("service=%s msg=starting addr=%s database_api_url=%s", serviceName, addr, databaseURL)
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

func (application *backendApp) handleHealth(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok", "service": application.serviceName})
}

func (application *backendApp) handleOK(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	err := application.createDatabaseEvent(databaseEventRequest{
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

	err := application.createDatabaseEvent(databaseEventRequest{
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

func (application *backendApp) createDatabaseEvent(payload databaseEventRequest) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/events", application.databaseURL), bytes.NewReader(jsonPayload))
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
