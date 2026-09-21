package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/capybara-brain346/jevtrieval/internal/config"
	"github.com/capybara-brain346/jevtrieval/internal/httpapi"
	"github.com/capybara-brain346/jevtrieval/internal/openrouter"
	"github.com/capybara-brain346/jevtrieval/internal/pipeline"
	"github.com/capybara-brain346/jevtrieval/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	database, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	if err := database.RecoverInterrupted(ctx); err != nil {
		log.Fatal(err)
	}
	client := openrouter.New(openrouter.Config{
		APIKey: cfg.OpenRouterAPIKey, BaseURL: cfg.OpenRouterBaseURL, Referer: cfg.OpenRouterHTTPReferer, AppName: cfg.OpenRouterAppName,
		QuestionModel: cfg.OpenRouterQuestionModel, AnswerModel: cfg.OpenRouterAnswerModel,
		EmbeddingModel: cfg.OpenRouterEmbeddingModel, JevModel: cfg.OpenRouterJevModel,
	})
	runner := &pipeline.Pipeline{Store: database, OpenRouter: client, Config: pipeline.Config{
		RetrievalLimit: cfg.RetrievalLimit, JevConcurrency: cfg.JevConcurrency, CoverageThreshold: cfg.JevCoverageThreshold,
		QuestionModel: cfg.OpenRouterQuestionModel, AnswerModel: cfg.OpenRouterAnswerModel,
		EmbeddingModel: cfg.OpenRouterEmbeddingModel, JevModel: cfg.OpenRouterJevModel,
	}}
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.New(database, runner, cfg.AllowedOrigin), ReadHeaderTimeout: 10 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("jevtrieval listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
