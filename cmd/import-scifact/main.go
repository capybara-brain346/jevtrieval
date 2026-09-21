package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/capybara-brain346/jevtrieval/internal/config"
	"github.com/capybara-brain346/jevtrieval/internal/openrouter"
	"github.com/capybara-brain346/jevtrieval/internal/store"
)

type sourceDocument struct {
	DocID string `json:"doc_id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type hfRowsResponse struct {
	Rows []struct {
		Row sourceDocument `json:"row"`
	} `json:"rows"`
	Error string `json:"error"`
}

func main() {
	var endpoint, localFile string
	var batchSize int
	var replace bool
	flag.StringVar(&endpoint, "url", "https://datasets-server.huggingface.co/rows", "Hugging Face datasets-server rows endpoint")
	flag.StringVar(&localFile, "file", "data/scifact/docs.jsonl", "optional local SciFact JSONL fallback")
	flag.IntVar(&batchSize, "batch", 64, "documents per embedding request")
	flag.BoolVar(&replace, "replace", false, "replace embeddings produced by another model")
	flag.Parse()
	if batchSize < 1 {
		fmt.Fprintln(os.Stderr, "-batch must be positive")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	ctx := context.Background()
	database, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal(err)
	}
	defer database.Close()
	client := openrouter.New(openrouter.Config{APIKey: cfg.OpenRouterAPIKey, BaseURL: cfg.OpenRouterBaseURL, Referer: cfg.OpenRouterHTTPReferer, AppName: cfg.OpenRouterAppName, EmbeddingModel: cfg.OpenRouterEmbeddingModel})

	documents, err := fetchDocuments(ctx, endpoint)
	if err != nil {
		if localFile == "" {
			fatal(err)
		}
		fmt.Fprintf(os.Stderr, "datasets-server unavailable (%v); reading %s\n", err, localFile)
		documents, err = readLocalDocuments(localFile)
		if err != nil {
			fatal(err)
		}
	}
	if len(documents) == 0 {
		fatal(fmt.Errorf("SciFact document source was empty"))
	}
	if err := validateDocuments(documents); err != nil {
		fatal(err)
	}
	ids := make([]string, len(documents))
	for i, document := range documents {
		ids[i] = document.DocID
	}
	models, err := database.DocumentModels(ctx, ids)
	if err != nil {
		fatal(err)
	}
	pending := make([]store.Document, 0, len(documents))
	skipped := 0
	for _, document := range documents {
		model, exists := models[document.DocID]
		if exists && model == cfg.OpenRouterEmbeddingModel {
			skipped++
			continue
		}
		if exists && !replace {
			fatal(fmt.Errorf("document %s has embedding model %q; rerun with -replace to replace it", document.DocID, model))
		}
		pending = append(pending, store.Document{DocID: document.DocID, Title: document.Title, Body: document.Text})
	}

	imported := 0
	failed := 0
	for start := 0; start < len(pending); start += batchSize {
		end := start + batchSize
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[start:end]
		texts := make([]string, len(batch))
		for i := range batch {
			texts[i] = batch[i].Body
		}
		vectors, err := client.Embed(ctx, texts)
		if err != nil {
			failed += len(batch)
			fmt.Fprintf(os.Stderr, "embedding batch %d-%d failed: %v\n", start, end-1, err)
			continue
		}
		if err := database.UpsertDocuments(ctx, batch, vectors, cfg.OpenRouterEmbeddingModel, replace); err != nil {
			failed += len(batch)
			fmt.Fprintf(os.Stderr, "database batch %d-%d failed: %v\n", start, end-1, err)
			continue
		}
		imported += len(batch)
		fmt.Printf("embedded %d/%d\n", imported, len(pending))
	}
	fmt.Printf("imported=%d skipped=%d failed=%d total=%d\n", imported, skipped, failed, len(documents))
	if failed > 0 {
		os.Exit(1)
	}
}

func fetchDocuments(ctx context.Context, endpoint string) ([]sourceDocument, error) {
	result := make([]sourceDocument, 0, 5183)
	for offset := 0; ; offset += 100 {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, err
		}
		query := u.Query()
		query.Set("dataset", "irds/beir_scifact")
		query.Set("config", "docs")
		query.Set("split", "docs")
		query.Set("offset", fmt.Sprint(offset))
		query.Set("length", "100")
		u.RawQuery = query.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		response.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("datasets-server returned %s: %s", response.Status, strings.TrimSpace(string(body)))
		}
		var page hfRowsResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		if page.Error != "" {
			return nil, fmt.Errorf("datasets-server returned an error: %s", page.Error)
		}
		for _, row := range page.Rows {
			result = append(result, row.Row)
		}
		if len(page.Rows) == 0 || len(page.Rows) < 100 {
			return result, nil
		}
	}
}

func validateDocuments(documents []sourceDocument) error {
	seen := make(map[string]struct{}, len(documents))
	for i, document := range documents {
		if strings.TrimSpace(document.DocID) == "" || strings.TrimSpace(document.Title) == "" || strings.TrimSpace(document.Text) == "" {
			return fmt.Errorf("invalid document %d (%q): doc_id, title, and text are required", i, document.DocID)
		}
		if _, exists := seen[document.DocID]; exists {
			return fmt.Errorf("duplicate document id %q", document.DocID)
		}
		seen[document.DocID] = struct{}{}
	}
	return nil
}

func readLocalDocuments(path string) ([]sourceDocument, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	var result []sourceDocument
	line := 0
	for scanner.Scan() {
		line++
		var document sourceDocument
		if err := json.Unmarshal(scanner.Bytes(), &document); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, line, err)
		}
		result = append(result, document)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
