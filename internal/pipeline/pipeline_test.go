package pipeline

import (
	"strings"
	"testing"

	"github.com/capybara-brain346/jevtrieval/internal/store"
)

func TestFinalPromptContainsOnlySelectedEvidence(t *testing.T) {
	prompt := finalPrompt(
		"What happened?",
		[]store.Question{{ID: 1, Text: "Does it contain evidence?"}},
		[]store.Document{
			{DocID: "selected", Title: "Selected", Body: "trusted finding"},
			{DocID: "discarded", Title: "Discarded", Body: "should not be sent"},
		},
		Selection{Evidence: []store.Evidence{{QuestionID: 1, DocID: "selected", Probability: 0.9}}},
	)
	if !strings.Contains(prompt, "trusted finding") || strings.Contains(prompt, "should not be sent") {
		t.Fatalf("prompt contained the wrong evidence: %s", prompt)
	}
	if !strings.Contains(prompt, "q_1 -> [selected]: 0.9000") {
		t.Fatalf("prompt omitted the question-to-document probability: %s", prompt)
	}
}
