package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

type DecisionDocument struct {
	DocID string
	Title string
	Text  string
}

type DecisionQuestion struct {
	ID   int64
	Text string
}

type decisionRequest struct {
	Model     string                      `json:"model"`
	State     decisionState               `json:"state"`
	Questions map[string]decisionQuestion `json:"questions"`
}

type decisionState struct {
	DocID string `json:"doc_id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type decisionQuestion struct {
	Type         string           `json:"type"`
	Instructions string           `json:"instructions"`
	Criteria     decisionCriteria `json:"criteria"`
}

type decisionCriteria struct {
	True  string `json:"true"`
	False string `json:"false"`
}

type decisionResponse struct {
	Answers map[string]struct {
		Type string          `json:"type"`
		Noul json.RawMessage `json:"noul"`
	} `json:"answers"`
}

func (c *Client) Decide(ctx context.Context, document DecisionDocument, questions []DecisionQuestion) (map[int64]float64, error) {
	if len(questions) == 0 {
		return nil, fmt.Errorf("decision questions are empty")
	}
	request := decisionRequest{
		Model:     c.config.JevModel,
		State:     decisionState{DocID: document.DocID, Title: document.Title, Text: document.Text},
		Questions: make(map[string]decisionQuestion, len(questions)),
	}
	for index, question := range questions {
		request.Questions[fmt.Sprintf("q_%d", index)] = decisionQuestion{
			Type:         "noul",
			Instructions: question.Text,
			Criteria: decisionCriteria{
				True:  "Substantive evidence relevant to the requirement is present.",
				False: "The requirement is absent or lacks useful evidence.",
			},
		}
	}
	var response decisionResponse
	if err := c.doJSON(ctx, "/alpha/decisions", request, &response); err != nil {
		return nil, fmt.Errorf("evaluate document %s: %w", document.DocID, err)
	}
	if len(response.Answers) != len(questions) {
		return nil, fmt.Errorf("decision response contained %d answers for %d questions", len(response.Answers), len(questions))
	}
	result := make(map[int64]float64, len(questions))
	for index, question := range questions {
		key := fmt.Sprintf("q_%d", index)
		answer, ok := response.Answers[key]
		if !ok {
			return nil, fmt.Errorf("decision response is missing %s", key)
		}
		if answer.Type != "noul" {
			return nil, fmt.Errorf("decision %s has type %q, want noul", key, answer.Type)
		}
		if len(answer.Noul) == 0 || string(answer.Noul) == "null" {
			return nil, fmt.Errorf("decision %s has invalid probability", key)
		}
		var probability float64
		if err := json.Unmarshal(answer.Noul, &probability); err != nil {
			return nil, fmt.Errorf("decision %s has invalid probability: %w", key, err)
		}
		if math.IsNaN(probability) || math.IsInf(probability, 0) || finiteProbability(probability) != nil {
			return nil, fmt.Errorf("decision %s has probability %v outside [0,1]", key, probability)
		}
		result[question.ID] = probability
	}
	return result, nil
}
