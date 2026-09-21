package openrouter

import (
	"context"
	"fmt"
	"math"
	"sort"
)

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func (c *Client) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("embedding input is empty")
	}
	var response embeddingResponse
	if err := c.doJSON(ctx, "/v1/embeddings", embeddingRequest{Model: c.config.EmbeddingModel, Input: texts}, &response); err != nil {
		return nil, fmt.Errorf("create embeddings: %w", err)
	}
	if len(response.Data) != len(texts) {
		return nil, fmt.Errorf("embedding response contained %d vectors for %d inputs", len(response.Data), len(texts))
	}
	sort.Slice(response.Data, func(i, j int) bool { return response.Data[i].Index < response.Data[j].Index })
	vectors := make([][]float64, len(texts))
	dimension := 0
	for i, item := range response.Data {
		if item.Index != i {
			return nil, fmt.Errorf("embedding response index %d at position %d", item.Index, i)
		}
		if len(item.Embedding) == 0 {
			return nil, fmt.Errorf("embedding %d is empty", i)
		}
		if dimension == 0 {
			dimension = len(item.Embedding)
		} else if len(item.Embedding) != dimension {
			return nil, fmt.Errorf("embedding %d has dimension %d, expected %d", i, len(item.Embedding), dimension)
		}
		for _, value := range item.Embedding {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("embedding %d contains a non-finite value", i)
			}
		}
		vectors[i] = item.Embedding
	}
	return vectors, nil
}
