package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/capybara-brain346/jevtrieval/internal/store"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) events(w http.ResponseWriter, r *http.Request, runID int64) {
	if err := h.store.EnsureRun(r.Context(), runID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
		} else {
			writeError(w, http.StatusInternalServerError, "could not read run")
		}
		return
	}
	after := maxEventID(parseEventID(r.Header.Get("Last-Event-ID")), parseEventID(r.URL.Query().Get("after")))
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	poll := time.NewTicker(200 * time.Millisecond)
	defer poll.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		events, err := h.store.EventsAfter(r.Context(), runID, after)
		if err != nil {
			return
		}
		for _, event := range events {
			if err := writeEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
			after = event.ID
			if event.EventType == "run.failed" || isCompletedStatus(event) {
				return
			}
		}
		terminal, err := h.store.IsTerminal(r.Context(), runID)
		if err != nil || terminal {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-poll.C:
		case <-heartbeat.C:
			if _, err := w.Write([]byte(": heartbeat\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeEvent(w http.ResponseWriter, event store.RunEvent) error {
	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage("null")
	}
	if !json.Valid(payload) {
		return fmt.Errorf("invalid event payload")
	}
	_, err := w.Write([]byte("id: " + strconv.FormatInt(event.ID, 10) + "\nevent: " + event.EventType + "\ndata: " + string(payload) + "\n\n"))
	return err
}

func isCompletedStatus(event store.RunEvent) bool {
	if event.EventType != "run.status" {
		return false
	}
	var payload struct {
		Status string `json:"status"`
	}
	return json.Unmarshal(event.Payload, &payload) == nil && (payload.Status == "completed" || payload.Status == "failed")
}

func parseEventID(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 0 {
		return 0
	}
	return id
}

func maxEventID(left, right int64) int64 {
	if right > left {
		return right
	}
	return left
}
