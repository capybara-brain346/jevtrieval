package pipeline

import (
	"math"
	"sort"

	"github.com/capybara-brain346/jevtrieval/internal/store"
)

type Selection struct {
	Evidence []store.Evidence
	Gaps     []store.CoverageGap
}

func SelectEvidence(questions []store.Question, documents []store.Document, judgments []store.Judgment, threshold float32) Selection {
	ranks := make(map[string]int, len(documents))
	for _, document := range documents {
		ranks[document.DocID] = document.Rank
	}
	byQuestion := make(map[int64][]store.Judgment)
	for _, judgment := range judgments {
		if judgment.Probability != nil {
			byQuestion[judgment.QuestionID] = append(byQuestion[judgment.QuestionID], judgment)
		}
	}
	selection := Selection{}
	for _, question := range questions {
		best := store.Judgment{}
		bestProbability := float32(0)
		found := false
		for _, judgment := range byQuestion[question.ID] {
			if judgment.Probability == nil || math.IsNaN(float64(*judgment.Probability)) {
				continue
			}
			probability := *judgment.Probability
			if !found || probability > bestProbability || (probability == bestProbability && ranks[judgment.DocID] < ranks[best.DocID]) {
				best, bestProbability, found = judgment, probability, true
			}
		}
		if found && bestProbability >= threshold {
			selection.Evidence = append(selection.Evidence, store.Evidence{QuestionID: question.ID, DocID: best.DocID, Probability: bestProbability})
		} else {
			selection.Gaps = append(selection.Gaps, store.CoverageGap{QuestionID: question.ID, BestProbability: bestProbability})
		}
	}
	sort.Slice(selection.Evidence, func(i, j int) bool { return selection.Evidence[i].QuestionID < selection.Evidence[j].QuestionID })
	sort.Slice(selection.Gaps, func(i, j int) bool { return selection.Gaps[i].QuestionID < selection.Gaps[j].QuestionID })
	return selection
}
