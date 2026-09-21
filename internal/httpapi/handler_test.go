package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthAndExactOriginCORS(t *testing.T) {
	handler := New(nil, nil, "http://localhost:3000")
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("status=%d headers=%v", response.Code, response.Header())
	}

	request = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "http://evil.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d", response.Code)
	}
}
