package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL              string
	OpenRouterAPIKey         string
	OpenRouterBaseURL        string
	OpenRouterEmbeddingModel string
	OpenRouterQuestionModel  string
	OpenRouterAnswerModel    string
	OpenRouterJevModel       string
	OpenRouterHTTPReferer    string
	OpenRouterAppName        string
	RetrievalLimit           int
	JevConcurrency           int
	JevCoverageThreshold     float32
	HTTPAddr                 string
	AllowedOrigin            string
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		OpenRouterAPIKey:         os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterBaseURL:        envOr("OPENROUTER_BASE_URL", "https://openrouter.ai/api"),
		OpenRouterEmbeddingModel: os.Getenv("OPENROUTER_EMBEDDING_MODEL"),
		OpenRouterQuestionModel:  os.Getenv("OPENROUTER_QUESTION_MODEL"),
		OpenRouterAnswerModel:    os.Getenv("OPENROUTER_ANSWER_MODEL"),
		OpenRouterJevModel:       envOr("OPENROUTER_JEV_MODEL", "~typesafe/jev-latest"),
		OpenRouterHTTPReferer:    os.Getenv("OPENROUTER_HTTP_REFERER"),
		OpenRouterAppName:        envOr("OPENROUTER_APP_NAME", "jevtrieval"),
		HTTPAddr:                 envOr("HTTP_ADDR", ":8080"),
		AllowedOrigin:            os.Getenv("ALLOWED_ORIGIN"),
	}

	var err error
	if c.RetrievalLimit, err = positiveInt("RETRIEVAL_LIMIT", 50); err != nil {
		return Config{}, err
	}
	if c.JevConcurrency, err = positiveInt("JEV_CONCURRENCY", 8); err != nil {
		return Config{}, err
	}
	if c.JevCoverageThreshold, err = probability("JEV_COVERAGE_THRESHOLD", 0.70); err != nil {
		return Config{}, err
	}
	for name, value := range map[string]string{
		"DATABASE_URL":               c.DatabaseURL,
		"OPENROUTER_API_KEY":         c.OpenRouterAPIKey,
		"OPENROUTER_EMBEDDING_MODEL": c.OpenRouterEmbeddingModel,
		"OPENROUTER_QUESTION_MODEL":  c.OpenRouterQuestionModel,
		"OPENROUTER_ANSWER_MODEL":    c.OpenRouterAnswerModel,
		"ALLOWED_ORIGIN":             c.AllowedOrigin,
	} {
		if strings.TrimSpace(value) == "" {
			return Config{}, fmt.Errorf("%s is required", name)
		}
	}
	return c, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func positiveInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return n, nil
}

func probability(name string, fallback float32) (float32, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.ParseFloat(value, 32)
	if err != nil || n < 0 || n > 1 {
		return 0, fmt.Errorf("%s must be between 0 and 1", name)
	}
	return float32(n), nil
}
