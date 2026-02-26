package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/chaisql/chai"
)

type event struct {
	ID        int    `json:"id"`
	Source    string `json:"source"`
	Method    string `json:"method"`
	Route     string `json:"route"`
	Status    int    `json:"status"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
}

type createEventRequest struct {
	Source  string `json:"source"`
	Method  string `json:"method"`
	Route   string `json:"route"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type note struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type createNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type app struct {
	db          *sql.DB
	serviceName string
}

func main() {
	addr := envOrDefault("DATABASE_ADDR", ":8082")
	databaseFile := envOrDefault("DATABASE_FILE", "/var/lib/chai/eventsdb")
	serviceName := envOrDefault("SERVICE_NAME", "database")

	if databaseFile != ":memory:" {
		err := os.MkdirAll(filepath.Dir(databaseFile), 0o755)
		if err != nil {
			log.Fatalf("service=%s msg=failed_to_prepare_db_directory err=%v", serviceName, err)
		}
	}

	db, err := sql.Open("chai", databaseFile)
	if err != nil {
		log.Fatalf("service=%s msg=failed_to_open_db err=%v", serviceName, err)
	}
	defer db.Close()

	err = ensureSchema(db)
	if err != nil {
		log.Fatalf("service=%s msg=failed_to_ensure_schema err=%v", serviceName, err)
	}

	application := &app{
		db:          db,
		serviceName: serviceName,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", application.handleHealth)
	mux.HandleFunc("/events", application.handleEvents)
	mux.HandleFunc("/events/", application.handleEventByID)
	mux.HandleFunc("/notes/export.md", application.exportNotesMarkdown)
	mux.HandleFunc("/notes", application.handleNotes)
	mux.HandleFunc("/notes/", application.handleNoteByID)

	handler := loggingMiddleware(serviceName, mux)
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("service=%s msg=starting addr=%s database_file=%s", serviceName, addr, databaseFile)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("service=%s msg=listen_failed err=%v", serviceName, err)
		}
	}()

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-signalChannel

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = server.Shutdown(shutdownContext)
	if err != nil {
		log.Printf("service=%s msg=shutdown_failed err=%v", serviceName, err)
	}

	log.Printf("service=%s msg=shutdown_complete", serviceName)
}

func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY,
			source TEXT NOT NULL,
			method TEXT NOT NULL,
			route TEXT NOT NULL,
			status INTEGER NOT NULL,
			message TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
	`)
	return err
}

func (application *app) handleHealth(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": application.serviceName,
	})
}

func (application *app) handleEvents(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		application.listEvents(response, request)
	case http.MethodPost:
		application.createEvent(response, request)
	default:
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (application *app) handleEventByID(response http.ResponseWriter, request *http.Request) {
	id, err := parseIDFromPath(request.URL.Path, "/events/")
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid event id")
		return
	}

	switch request.Method {
	case http.MethodGet:
		application.getEvent(response, id)
	case http.MethodDelete:
		application.deleteEvent(response, id)
	default:
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (application *app) listEvents(response http.ResponseWriter, request *http.Request) {
	limit := 50
	queryLimit := request.URL.Query().Get("limit")
	if queryLimit != "" {
		parsedLimit, err := strconv.Atoi(queryLimit)
		if err != nil || parsedLimit <= 0 || parsedLimit > 500 {
			writeError(response, http.StatusBadRequest, "limit must be between 1 and 500")
			return
		}
		limit = parsedLimit
	}

	rows, err := application.db.Query(
		"SELECT id, source, method, route, status, message, created_at FROM events ORDER BY id DESC LIMIT $1",
		limit,
	)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to query events")
		return
	}
	defer rows.Close()

	var events []event
	for rows.Next() {
		var row event
		err = rows.Scan(&row.ID, &row.Source, &row.Method, &row.Route, &row.Status, &row.Message, &row.CreatedAt)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to scan event")
			return
		}
		events = append(events, row)
	}

	err = rows.Err()
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to read rows")
		return
	}

	writeJSON(response, http.StatusOK, map[string]any{
		"count":  len(events),
		"events": events,
	})
}

func (application *app) getEvent(response http.ResponseWriter, id int) {
	var stored event
	err := application.db.QueryRow(
		"SELECT id, source, method, route, status, message, created_at FROM events WHERE id = $1",
		id,
	).Scan(&stored.ID, &stored.Source, &stored.Method, &stored.Route, &stored.Status, &stored.Message, &stored.CreatedAt)
	if err == sql.ErrNoRows {
		writeError(response, http.StatusNotFound, "event not found")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to load event")
		return
	}

	writeJSON(response, http.StatusOK, stored)
}

func (application *app) createEvent(response http.ResponseWriter, request *http.Request) {
	var input createEventRequest
	err := json.NewDecoder(request.Body).Decode(&input)
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if input.Source == "" {
		input.Source = "unknown"
	}
	if input.Method == "" {
		input.Method = "GET"
	}
	if input.Route == "" {
		input.Route = "/"
	}
	if input.Status == 0 {
		input.Status = http.StatusOK
	}
	if input.Message == "" {
		input.Message = "request completed"
	}

	nextID, err := application.nextEventID()
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to allocate event id")
		return
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	_, err = application.db.Exec(
		"INSERT INTO events (id, source, method, route, status, message, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		nextID,
		input.Source,
		input.Method,
		input.Route,
		input.Status,
		input.Message,
		createdAt,
	)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create event")
		return
	}

	writeJSON(response, http.StatusCreated, event{
		ID:        nextID,
		Source:    input.Source,
		Method:    input.Method,
		Route:     input.Route,
		Status:    input.Status,
		Message:   input.Message,
		CreatedAt: createdAt,
	})
}

func (application *app) deleteEvent(response http.ResponseWriter, id int) {
	_, err := application.db.Exec("DELETE FROM events WHERE id = $1", id)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to delete event")
		return
	}

	response.WriteHeader(http.StatusNoContent)
}

func (application *app) nextEventID() (int, error) {
	var nextID int
	err := application.db.QueryRow("SELECT COALESCE(MAX(id), 0) + 1 FROM events").Scan(&nextID)
	return nextID, err
}

func (application *app) handleNotes(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		application.listNotes(response)
	case http.MethodPost:
		application.createNote(response, request)
	default:
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (application *app) handleNoteByID(response http.ResponseWriter, request *http.Request) {
	id, err := parseIDFromPath(request.URL.Path, "/notes/")
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid note id")
		return
	}

	switch request.Method {
	case http.MethodGet:
		application.getNote(response, id)
	case http.MethodPut:
		application.updateNote(response, request, id)
	case http.MethodDelete:
		application.deleteNote(response, id)
	default:
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (application *app) listNotes(response http.ResponseWriter) {
	rows, err := application.db.Query(
		"SELECT id, title, content, created_at, updated_at FROM notes ORDER BY id DESC",
	)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to query notes")
		return
	}
	defer rows.Close()

	var notes []note
	for rows.Next() {
		var row note
		err = rows.Scan(&row.ID, &row.Title, &row.Content, &row.CreatedAt, &row.UpdatedAt)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to scan note")
			return
		}
		notes = append(notes, row)
	}

	err = rows.Err()
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to read note rows")
		return
	}

	writeJSON(response, http.StatusOK, map[string]any{
		"count": len(notes),
		"notes": notes,
	})
}

func (application *app) getNote(response http.ResponseWriter, id int) {
	var stored note
	err := application.db.QueryRow(
		"SELECT id, title, content, created_at, updated_at FROM notes WHERE id = $1",
		id,
	).Scan(&stored.ID, &stored.Title, &stored.Content, &stored.CreatedAt, &stored.UpdatedAt)
	if err == sql.ErrNoRows {
		writeError(response, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to load note")
		return
	}

	writeJSON(response, http.StatusOK, stored)
}

func (application *app) createNote(response http.ResponseWriter, request *http.Request) {
	var input createNoteRequest
	err := json.NewDecoder(request.Body).Decode(&input)
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Untitled Note"
	}

	nextID, err := application.nextNoteID()
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to allocate note id")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = application.db.Exec(
		"INSERT INTO notes (id, title, content, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)",
		nextID,
		title,
		input.Content,
		now,
		now,
	)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create note")
		return
	}

	writeJSON(response, http.StatusCreated, note{
		ID:        nextID,
		Title:     title,
		Content:   input.Content,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (application *app) updateNote(response http.ResponseWriter, request *http.Request, id int) {
	var input createNoteRequest
	err := json.NewDecoder(request.Body).Decode(&input)
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Untitled Note"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = application.db.Exec(
		"UPDATE notes SET title = $1, content = $2, updated_at = $3 WHERE id = $4",
		title,
		input.Content,
		now,
		id,
	)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to update note")
		return
	}

	application.getNote(response, id)
}

func (application *app) deleteNote(response http.ResponseWriter, id int) {
	_, err := application.db.Exec("DELETE FROM notes WHERE id = $1", id)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to delete note")
		return
	}

	response.WriteHeader(http.StatusNoContent)
}

func (application *app) nextNoteID() (int, error) {
	var nextID int
	err := application.db.QueryRow("SELECT COALESCE(MAX(id), 0) + 1 FROM notes").Scan(&nextID)
	return nextID, err
}

func (application *app) exportNotesMarkdown(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(response, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rows, err := application.db.Query(
		"SELECT id, title, content, created_at, updated_at FROM notes ORDER BY id ASC",
	)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to query notes for export")
		return
	}
	defer rows.Close()

	var notes []note
	for rows.Next() {
		var row note
		err = rows.Scan(&row.ID, &row.Title, &row.Content, &row.CreatedAt, &row.UpdatedAt)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to scan note for export")
			return
		}
		notes = append(notes, row)
	}

	err = rows.Err()
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to read exported note rows")
		return
	}

	var builder strings.Builder
	builder.WriteString("# Workshop Notes\n\n")
	builder.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	if len(notes) == 0 {
		builder.WriteString("_No notes saved yet._\n")
	} else {
		for _, savedNote := range notes {
			builder.WriteString(fmt.Sprintf("## %s\n\n", savedNote.Title))
			builder.WriteString(fmt.Sprintf("- ID: %d\n", savedNote.ID))
			builder.WriteString(fmt.Sprintf("- Created: %s\n", savedNote.CreatedAt))
			builder.WriteString(fmt.Sprintf("- Updated: %s\n\n", savedNote.UpdatedAt))
			if strings.TrimSpace(savedNote.Content) == "" {
				builder.WriteString("(empty note)\n\n")
			} else {
				builder.WriteString(savedNote.Content)
				builder.WriteString("\n\n")
			}
		}
	}

	response.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	response.Header().Set("Content-Disposition", "attachment; filename=workshop-notes.md")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(builder.String()))
}

func parseIDFromPath(path string, prefix string) (int, error) {
	rawID := strings.TrimPrefix(path, prefix)
	if rawID == "" || strings.Contains(rawID, "/") {
		return 0, fmt.Errorf("invalid id")
	}

	id, err := strconv.Atoi(rawID)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}

	return id, nil
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
