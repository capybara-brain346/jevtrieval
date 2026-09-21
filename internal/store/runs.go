package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type RunConfig struct {
	QuestionModel     string
	AnswerModel       string
	EmbeddingModel    string
	JevModel          string
	CoverageThreshold float32
}

func (s *Store) CreateRun(ctx context.Context, query string, config RunConfig) (Run, error) {
	var run Run
	err := withTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var answer, failure *string
		err := tx.QueryRow(ctx, `
			INSERT INTO runs(query, status, question_model, answer_model, embedding_model, jev_model, coverage_threshold)
			VALUES($1, 'queued', $2, $3, $4, $5, $6)
			RETURNING id, query, status, answer, error, warning_count, question_model, answer_model, embedding_model, jev_model, coverage_threshold`,
			query, config.QuestionModel, config.AnswerModel, config.EmbeddingModel, config.JevModel, config.CoverageThreshold).
			Scan(&run.ID, &run.Query, &run.Status, &answer, &failure, &run.WarningCount, &run.QuestionModel, &run.AnswerModel, &run.EmbeddingModel, &run.JevModel, &run.CoverageThreshold)
		if err != nil {
			return err
		}
		run.Answer, run.Error = answer, failure
		return appendEventTx(ctx, tx, run.ID, "run.created", map[string]any{"run": run})
	})
	return run, err
}

func (s *Store) SetStatus(ctx context.Context, runID int64, status string) error {
	return withTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE runs SET status=$2, updated_at=now() WHERE id=$1`, runID, status); err != nil {
			return err
		}
		return appendEventTx(ctx, tx, runID, "run.status", map[string]string{"status": status})
	})
}

func (s *Store) SaveQuestions(ctx context.Context, runID int64, texts []string) ([]Question, error) {
	questions := make([]Question, 0, len(texts))
	err := withTx(ctx, s.Pool, func(tx pgx.Tx) error {
		for ordinal, text := range texts {
			var question Question
			if err := tx.QueryRow(ctx, `INSERT INTO questions(run_id, ordinal, text) VALUES($1,$2,$3) RETURNING id, ordinal, text`, runID, ordinal, text).Scan(&question.ID, &question.Ordinal, &question.Text); err != nil {
				return err
			}
			questions = append(questions, question)
		}
		if _, err := tx.Exec(ctx, `UPDATE runs SET updated_at=now() WHERE id=$1`, runID); err != nil {
			return err
		}
		return appendEventTx(ctx, tx, runID, "questions.generated", map[string]any{"questions": questions})
	})
	return questions, err
}

func (s *Store) SaveJudgment(ctx context.Context, runID int64, judgment Judgment) error {
	return withTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var probability any
		if judgment.Probability != nil {
			probability = *judgment.Probability
		}
		var failure any
		if judgment.Error != nil {
			failure = *judgment.Error
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO judgments(run_id, question_id, doc_id, probability, error) VALUES($1,$2,$3,$4,$5)
			ON CONFLICT(run_id, question_id, doc_id) DO UPDATE SET probability=EXCLUDED.probability, error=EXCLUDED.error`,
			runID, judgment.QuestionID, judgment.DocID, probability, failure); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE runs SET updated_at=now() WHERE id=$1`, runID); err != nil {
			return err
		}
		if judgment.Probability != nil {
			return appendEventTx(ctx, tx, runID, "judgment.completed", map[string]any{"question_id": judgment.QuestionID, "doc_id": judgment.DocID, "probability": *judgment.Probability})
		}
		return appendEventTx(ctx, tx, runID, "judgment.failed", map[string]any{"question_id": judgment.QuestionID, "doc_id": judgment.DocID, "error": stringValue(judgment.Error)})
	})
}

func (s *Store) SaveEvidence(ctx context.Context, runID int64, evidence []Evidence, gaps []CoverageGap) error {
	if evidence == nil {
		evidence = []Evidence{}
	}
	if gaps == nil {
		gaps = []CoverageGap{}
	}
	return withTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM evidence WHERE run_id=$1`, runID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM coverage_gaps WHERE run_id=$1`, runID); err != nil {
			return err
		}
		for _, item := range evidence {
			if _, err := tx.Exec(ctx, `INSERT INTO evidence(run_id, question_id, doc_id, probability) VALUES($1,$2,$3,$4)`, runID, item.QuestionID, item.DocID, item.Probability); err != nil {
				return err
			}
		}
		for _, gap := range gaps {
			if _, err := tx.Exec(ctx, `INSERT INTO coverage_gaps(run_id, question_id, best_probability) VALUES($1,$2,$3)`, runID, gap.QuestionID, gap.BestProbability); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE runs SET updated_at=now() WHERE id=$1`, runID); err != nil {
			return err
		}
		return appendEventTx(ctx, tx, runID, "evidence.selected", map[string]any{"evidence": evidence, "gaps": gaps})
	})
}

func (s *Store) CompleteRun(ctx context.Context, runID int64, answer string, warnings int) error {
	return withTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE runs SET answer=$2, warning_count=$3, status='completed', error=NULL, updated_at=now(), completed_at=now() WHERE id=$1`, runID, answer, warnings); err != nil {
			return err
		}
		if err := appendEventTx(ctx, tx, runID, "answer.completed", map[string]string{"answer": answer}); err != nil {
			return err
		}
		return appendEventTx(ctx, tx, runID, "run.status", map[string]string{"status": "completed"})
	})
}

func (s *Store) FailRun(ctx context.Context, runID int64, failure string, warnings int) error {
	return withTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE runs SET status='failed', error=$2, warning_count=$3, updated_at=now(), completed_at=now() WHERE id=$1`, runID, failure, warnings); err != nil {
			return err
		}
		return appendEventTx(ctx, tx, runID, "run.failed", map[string]string{"error": failure})
	})
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
