package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestFetchDocumentsPaginatesRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("dataset") != "irds/beir_scifact" || r.URL.Query().Get("config") != "docs" || r.URL.Query().Get("split") != "docs" {
			t.Fatalf("unexpected dataset query: %s", r.URL.RawQuery)
		}
		offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
		if err != nil {
			t.Fatal(err)
		}
		length, err := strconv.Atoi(r.URL.Query().Get("length"))
		if err != nil || length != 100 {
			t.Fatalf("length = %q", r.URL.Query().Get("length"))
		}
		count := 1
		if offset == 0 {
			count = 100
		} else if offset != 100 {
			t.Fatalf("offset = %d", offset)
		}
		rows := make([]map[string]sourceDocument, count)
		for i := range rows {
			id := strconv.Itoa(offset + i)
			rows[i] = map[string]sourceDocument{"row": {DocID: id, Title: "title " + id, Text: "text " + id}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": rows})
	}))
	defer server.Close()

	documents, err := fetchDocuments(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 101 || documents[100].DocID != "100" {
		t.Fatalf("documents = %#v", documents)
	}
}

func TestFetchDocumentsReportsDatasetErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "dataset viewer disabled"})
	}))
	defer server.Close()

	if _, err := fetchDocuments(context.Background(), server.URL); err == nil {
		t.Fatal("expected dataset error")
	}
}

func TestValidateDocuments(t *testing.T) {
	valid := []sourceDocument{{DocID: "D1", Title: "Title", Text: "Text"}}
	if err := validateDocuments(valid); err != nil {
		t.Fatal(err)
	}
	for _, documents := range [][]sourceDocument{
		{{DocID: "", Title: "Title", Text: "Text"}},
		{{DocID: "D1", Title: " ", Text: "Text"}},
		{{DocID: "D1", Title: "Title", Text: "Text"}, {DocID: "D1", Title: "Other", Text: "Other"}},
	} {
		if err := validateDocuments(documents); err == nil {
			t.Fatalf("validateDocuments(%#v) succeeded", documents)
		}
	}
}
