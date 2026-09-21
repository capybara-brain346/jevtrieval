package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	APIKey         string
	BaseURL        string
	Referer        string
	AppName        string
	QuestionModel  string
	AnswerModel    string
	EmbeddingModel string
	JevModel       string
}

type Client struct {
	config Config
	http   *http.Client
	sleep  func(context.Context, time.Duration) error
}

func New(config Config) *Client {
	return &Client{
		config: config,
		http:   &http.Client{Timeout: 90 * time.Second},
		sleep:  sleep,
	}
}

func (c *Client) Config() Config { return c.config }

func (c *Client) endpoint(path string) string {
	return strings.TrimRight(c.config.BaseURL, "/") + path
}

type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("OpenRouter request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("OpenRouter request failed with status %d: %s", e.StatusCode, e.Body)
}

func (c *Client) doJSON(ctx context.Context, path string, requestBody any, responseBody any) error {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshal OpenRouter request: %w", err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(path), bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
		req.Header.Set("Content-Type", "application/json")
		if c.config.Referer != "" {
			req.Header.Set("HTTP-Referer", c.config.Referer)
		}
		if c.config.AppName != "" {
			req.Header.Set("X-Title", c.config.AppName)
		}

		response, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("OpenRouter request: %w", err)
		}
		responseBytes, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		response.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read OpenRouter response: %w", readErr)
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			if err := json.Unmarshal(responseBytes, responseBody); err != nil {
				return fmt.Errorf("decode OpenRouter response: %w", err)
			}
			return nil
		}

		requestErr := &apiError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(responseBytes))}
		if !retryable(response.StatusCode) || attempt == 2 {
			return requestErr
		}
		if err := c.sleep(ctx, time.Duration(1<<attempt)*250*time.Millisecond); err != nil {
			return err
		}
	}
	return fmt.Errorf("OpenRouter request exhausted retries")
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func finiteProbability(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return fmt.Errorf("probability %v is outside [0,1]", value)
	}
	return nil
}
