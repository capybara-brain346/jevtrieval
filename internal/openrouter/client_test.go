package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(serverURL string) *Client {
	client := New(Config{APIKey: "secret", BaseURL: serverURL, QuestionModel: "questions", AnswerModel: "answers", EmbeddingModel: "embeddings", JevModel: "jev"})
	client.sleep = func(context.Context, time.Duration) error { return nil }
	return client
}

func TestClientParsesQuestionEmbeddingAndDecisionResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/chat/completions":
			var request chatRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.ResponseFormat == nil || request.ResponseFormat.Type != "json_schema" {
				t.Error("question request lacks JSON schema")
			}
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": `{"questions":["first requirement?","second requirement?"]}`}}}})
		case "/v1/embeddings":
			json.NewEncoder(w).Encode(map[string]any{"data": []any{
				map[string]any{"index": 1, "embedding": []float64{0.2, 0.3}},
				map[string]any{"index": 0, "embedding": []float64{0.1, 0.2}},
			}})
		case "/alpha/decisions":
			json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{
				"q_0": map[string]any{"type": "noul", "noul": 0.8},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := testClient(server.URL)
	questions, err := client.GenerateQuestions(context.Background(), "query")
	if err != nil {
		t.Fatal(err)
	}
	if len(questions) != 2 || questions[0] != "first requirement?" {
		t.Fatalf("questions = %#v", questions)
	}
	vectors, err := client.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if vectors[0][0] != 0.1 || vectors[1][0] != 0.2 {
		t.Fatalf("vectors = %#v", vectors)
	}
	decisions, err := client.Decide(context.Background(), DecisionDocument{DocID: "D1"}, []DecisionQuestion{{ID: 11, Text: "requirement?"}})
	if err != nil {
		t.Fatal(err)
	}
	if decisions[11] != 0.8 {
		t.Fatalf("decisions = %#v", decisions)
	}
}

func TestClientRetriesTransientErrors(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"index": 0, "embedding": []float64{1}}}})
	}))
	defer server.Close()
	vectors, err := testClient(server.URL).Embed(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 || vectors[0][0] != 1 {
		t.Fatalf("calls=%d vectors=%#v", calls.Load(), vectors)
	}
}

func TestClientRejectsInvalidDecisionProbability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{"q_1": map[string]any{"type": "noul", "noul": 1.2}}})
	}))
	defer server.Close()
	_, err := testClient(server.URL).Decide(context.Background(), DecisionDocument{DocID: "D1"}, []DecisionQuestion{{ID: 1, Text: "q"}})
	if err == nil {
		t.Fatal("expected invalid probability error")
	}
}
