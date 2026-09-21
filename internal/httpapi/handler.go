package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/capybara-brain346/jevtrieval/internal/pipeline"
	"github.com/capybara-brain346/jevtrieval/internal/store"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	store          *store.Store
	pipeline       *pipeline.Pipeline
	allowedOrigin  string
	maxQueryLength int
}

func New(st *store.Store, runner *pipeline.Pipeline, allowedOrigin string) http.Handler {
	return &Handler{store: st, pipeline: runner, allowedOrigin: allowedOrigin, maxQueryLength: 4000}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.cors(w, r) {
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.Path == "/healthz" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if r.URL.Path == "/v1/runs" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		h.createRun(w, r)
		return
	}
	prefix := "/v1/runs/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")
	if len(parts) < 1 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	runID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || runID < 1 {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		h.snapshot(w, r, runID)
		return
	}
	if len(parts) == 2 && parts[1] == "events" && r.Method == http.MethodGet {
		h.events(w, r, runID)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) cors(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if origin != h.allowedOrigin {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", h.allowedOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Last-Event-ID")
	w.Header().Add("Vary", "Origin")
	return true
}

type createRunRequest struct {
	Query string `json:"query"`
}

func (h *Handler) createRun(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(h.maxQueryLength+1024))
	var request createRunRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}
	if err := ensureEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "request body must contain one JSON object")
		return
	}
	query := strings.TrimSpace(request.Query)
	if query == "" {
		writeError(w, http.StatusBadRequest, "query must not be empty")
		return
	}
	if len([]rune(query)) > h.maxQueryLength {
		writeError(w, http.StatusBadRequest, "query is too long")
		return
	}
	if h.store == nil || h.pipeline == nil {
		writeError(w, http.StatusInternalServerError, "server is not configured")
		return
	}
	run, err := h.store.CreateRun(r.Context(), query, store.RunConfig{
		QuestionModel:     h.pipeline.Config.QuestionModel,
		AnswerModel:       h.pipeline.Config.AnswerModel,
		EmbeddingModel:    h.pipeline.Config.EmbeddingModel,
		JevModel:          h.pipeline.Config.JevModel,
		CoverageThreshold: h.pipeline.Config.CoverageThreshold,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create run")
		return
	}
	go h.pipeline.Run(context.Background(), run.ID, query)
	writeJSON(w, http.StatusOK, map[string]any{"id": run.ID, "status": run.Status})
}

func (h *Handler) snapshot(w http.ResponseWriter, r *http.Request, runID int64) {
	snapshot, err := h.store.Snapshot(r.Context(), runID)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read run")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("extra JSON")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, POST, OPTIONS")
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
