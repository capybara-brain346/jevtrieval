package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/capybara-brain346/jevtrieval/internal/config"
	"github.com/capybara-brain346/jevtrieval/internal/openrouter"
	"github.com/capybara-brain346/jevtrieval/internal/pipeline"
	"github.com/capybara-brain346/jevtrieval/internal/store"
)

type queryRecord struct {
	ID   string `json:"query_id"`
	Text string `json:"text"`
}
type qrelRecord struct {
	QueryID   string `json:"query_id"`
	DocID     string `json:"doc_id"`
	Relevance int    `json:"relevance"`
	Score     int    `json:"score"`
}

type metrics struct {
	Queries                  int     `json:"queries"`
	BaselineRecallAt5        float64 `json:"baseline_recall_at_5"`
	CandidateRecallAt50      float64 `json:"candidate_recall_at_50"`
	BaselineNDCG             float64 `json:"baseline_ndcg"`
	CandidateNDCG            float64 `json:"candidate_ndcg"`
	SelectedPrecision        float64 `json:"selected_evidence_precision"`
	SelectedRecall           float64 `json:"selected_evidence_recall"`
	AverageSelectedDocuments float64 `json:"average_selected_documents"`
	LatencyMilliseconds      float64 `json:"average_latency_ms"`
}

func main() {
	var queriesPath, qrelsPath string
	var sample int
	var runPipeline bool
	flag.StringVar(&queriesPath, "queries", "data/scifact/queries.jsonl", "SciFact query JSONL")
	flag.StringVar(&qrelsPath, "qrels", "data/scifact/qrels.jsonl", "SciFact qrels JSONL")
	flag.IntVar(&sample, "sample", 25, "number of queries to evaluate")
	flag.BoolVar(&runPipeline, "run-pipeline", false, "run the Jev pipeline for selected-evidence metrics")
	flag.Parse()
	if sample < 1 {
		fatal(fmt.Errorf("-sample must be positive"))
	}
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	queries, err := readJSONL[queryRecord](queriesPath)
	if err != nil {
		fatal(err)
	}
	qrels, err := readJSONL[qrelRecord](qrelsPath)
	if err != nil {
		fatal(err)
	}
	relevant := make(map[string]map[string]bool)
	for _, qrel := range qrels {
		if qrel.Relevance > 0 || qrel.Score > 0 {
			if relevant[qrel.QueryID] == nil {
				relevant[qrel.QueryID] = map[string]bool{}
			}
			relevant[qrel.QueryID][qrel.DocID] = true
		}
	}

	ctx := context.Background()
	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal(err)
	}
	defer db.Close()
	client := openrouter.New(openrouter.Config{APIKey: cfg.OpenRouterAPIKey, BaseURL: cfg.OpenRouterBaseURL, Referer: cfg.OpenRouterHTTPReferer, AppName: cfg.OpenRouterAppName, QuestionModel: cfg.OpenRouterQuestionModel, AnswerModel: cfg.OpenRouterAnswerModel, EmbeddingModel: cfg.OpenRouterEmbeddingModel, JevModel: cfg.OpenRouterJevModel})
	runner := &pipeline.Pipeline{Store: db, OpenRouter: client, Config: pipeline.Config{RetrievalLimit: cfg.RetrievalLimit, JevConcurrency: cfg.JevConcurrency, CoverageThreshold: cfg.JevCoverageThreshold, QuestionModel: cfg.OpenRouterQuestionModel, AnswerModel: cfg.OpenRouterAnswerModel, EmbeddingModel: cfg.OpenRouterEmbeddingModel, JevModel: cfg.OpenRouterJevModel}}
	result := metrics{}
	var selectedPrecision, selectedRecall, selectedDocs float64
	var selectedQueries int
	var latency time.Duration
	for _, query := range queries {
		if result.Queries >= sample {
			break
		}
		rels := relevant[query.ID]
		if len(rels) == 0 {
			continue
		}
		start := time.Now()
		vectors, err := client.Embed(ctx, []string{query.Text})
		if err != nil {
			fatal(err)
		}
		documents, err := db.SearchDocuments(ctx, vectors[0], cfg.OpenRouterEmbeddingModel, cfg.RetrievalLimit)
		if err != nil {
			fatal(err)
		}
		latency += time.Since(start)
		result.Queries++
		result.BaselineRecallAt5 += recall(documents[:min(5, len(documents))], rels)
		result.CandidateRecallAt50 += recall(documents, rels)
		result.BaselineNDCG += ndcg(documents[:min(5, len(documents))], rels)
		result.CandidateNDCG += ndcg(documents, rels)
		if runPipeline {
			run, err := db.CreateRun(ctx, query.Text, store.RunConfig{QuestionModel: cfg.OpenRouterQuestionModel, AnswerModel: cfg.OpenRouterAnswerModel, EmbeddingModel: cfg.OpenRouterEmbeddingModel, JevModel: cfg.OpenRouterJevModel, CoverageThreshold: cfg.JevCoverageThreshold})
			if err != nil {
				fatal(err)
			}
			runner.Run(ctx, run.ID, query.Text)
			snapshot, err := db.Snapshot(ctx, run.ID)
			if err != nil {
				fatal(err)
			}
			selected := map[string]bool{}
			for _, item := range snapshot.Evidence {
				selected[item.DocID] = true
			}
			selectedQueries++
			selectedDocs += float64(len(selected))
			hits := 0
			for docID := range selected {
				if rels[docID] {
					hits++
				}
			}
			selectedPrecision += ratio(float64(hits), float64(len(selected)))
			selectedRecall += ratio(float64(hits), float64(len(rels)))
		}
	}
	if result.Queries > 0 {
		result.BaselineRecallAt5 /= float64(result.Queries)
		result.CandidateRecallAt50 /= float64(result.Queries)
		result.BaselineNDCG /= float64(result.Queries)
		result.CandidateNDCG /= float64(result.Queries)
		result.LatencyMilliseconds = latency.Seconds() * 1000 / float64(result.Queries)
	}
	if selectedQueries > 0 {
		result.SelectedPrecision = selectedPrecision / float64(selectedQueries)
		result.SelectedRecall = selectedRecall / float64(selectedQueries)
		result.AverageSelectedDocuments = selectedDocs / float64(selectedQueries)
	}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(encoded))
}

func readJSONL[T any](path string) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 8<<20)
	var records []T
	for scanner.Scan() {
		var record T
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

func recall(documents []store.Document, relevant map[string]bool) float64 {
	hits := 0
	for _, document := range documents {
		if relevant[document.DocID] {
			hits++
		}
	}
	return ratio(float64(hits), float64(len(relevant)))
}
func ndcg(documents []store.Document, relevant map[string]bool) float64 {
	if len(relevant) == 0 {
		return 0
	}
	actual := 0.0
	for i, document := range documents {
		if relevant[document.DocID] {
			actual += 1 / math.Log2(float64(i+2))
		}
	}
	ideal := 0.0
	for i := 0; i < min(len(documents), len(relevant)); i++ {
		ideal += 1 / math.Log2(float64(i+2))
	}
	return ratio(actual, ideal)
}
func ratio(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
