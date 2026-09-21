package pipeline

import (
	"testing"

	"github.com/capybara-brain346/jevtrieval/internal/store"
)

func TestSelectEvidenceUsesThresholdAndVectorRank(t *testing.T) {
	questions := []store.Question{{ID: 1}, {ID: 2}, {ID: 3}}
	documents := []store.Document{{DocID: "best", Rank: 1}, {DocID: "tie", Rank: 2}, {DocID: "late", Rank: 3}}
	p := func(value float32) *float32 { return &value }
	judgments := []store.Judgment{
		{QuestionID: 1, DocID: "tie", Probability: p(0.8)},
		{QuestionID: 1, DocID: "best", Probability: p(0.8)},
		{QuestionID: 2, DocID: "late", Probability: p(0.69)},
		{QuestionID: 3, DocID: "best", Probability: p(0.2)},
	}
	selection := SelectEvidence(questions, documents, judgments, 0.7)
	if len(selection.Evidence) != 1 || selection.Evidence[0].DocID != "best" || selection.Evidence[0].QuestionID != 1 {
		t.Fatalf("evidence = %#v", selection.Evidence)
	}
	if len(selection.Gaps) != 2 || selection.Gaps[0].QuestionID != 2 || selection.Gaps[0].BestProbability != 0.69 {
		t.Fatalf("gaps = %#v", selection.Gaps)
	}
}
