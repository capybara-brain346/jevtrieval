package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (s *Store) Snapshot(ctx context.Context, runID int64) (Snapshot, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	snapshot := Snapshot{
		Questions: make([]Question, 0), Documents: make([]Document, 0), Judgments: make([]Judgment, 0),
		Evidence: make([]Evidence, 0), Gaps: make([]CoverageGap, 0),
	}
	var answer, failure *string
	if err := tx.QueryRow(ctx, `
		SELECT id, query, status, answer, error, warning_count, question_model, answer_model, embedding_model, jev_model, coverage_threshold
		FROM runs WHERE id=$1`, runID).Scan(
		&snapshot.Run.ID, &snapshot.Run.Query, &snapshot.Run.Status, &answer, &failure, &snapshot.Run.WarningCount,
		&snapshot.Run.QuestionModel, &snapshot.Run.AnswerModel, &snapshot.Run.EmbeddingModel, &snapshot.Run.JevModel, &snapshot.Run.CoverageThreshold,
	); err != nil {
		return Snapshot{}, err
	}
	snapshot.Run.Answer, snapshot.Run.Error = answer, failure

	questions, err := tx.Query(ctx, `SELECT id, ordinal, text FROM questions WHERE run_id=$1 ORDER BY ordinal`, runID)
	if err != nil {
		return Snapshot{}, err
	}
	for questions.Next() {
		var item Question
		if err := questions.Scan(&item.ID, &item.Ordinal, &item.Text); err != nil {
			questions.Close()
			return Snapshot{}, err
		}
		snapshot.Questions = append(snapshot.Questions, item)
	}
	if err := questions.Err(); err != nil {
		questions.Close()
		return Snapshot{}, err
	}
	questions.Close()

	documents, err := tx.Query(ctx, `
		SELECT d.doc_id, d.title, left(d.body, 280), rd.rank, rd.vector_score
		FROM run_documents rd JOIN documents d ON d.doc_id=rd.doc_id
		WHERE rd.run_id=$1 ORDER BY rd.rank`, runID)
	if err != nil {
		return Snapshot{}, err
	}
	for documents.Next() {
		var item Document
		if err := documents.Scan(&item.DocID, &item.Title, &item.TextPreview, &item.Rank, &item.VectorScore); err != nil {
			documents.Close()
			return Snapshot{}, err
		}
		snapshot.Documents = append(snapshot.Documents, item)
	}
	if err := documents.Err(); err != nil {
		documents.Close()
		return Snapshot{}, err
	}
	documents.Close()

	judgments, err := tx.Query(ctx, `SELECT question_id, doc_id, probability, error FROM judgments WHERE run_id=$1 ORDER BY question_id, doc_id`, runID)
	if err != nil {
		return Snapshot{}, err
	}
	for judgments.Next() {
		var item Judgment
		if err := judgments.Scan(&item.QuestionID, &item.DocID, &item.Probability, &item.Error); err != nil {
			judgments.Close()
			return Snapshot{}, err
		}
		snapshot.Judgments = append(snapshot.Judgments, item)
	}
	if err := judgments.Err(); err != nil {
		judgments.Close()
		return Snapshot{}, err
	}
	judgments.Close()

	evidence, err := tx.Query(ctx, `SELECT question_id, doc_id, probability FROM evidence WHERE run_id=$1 ORDER BY question_id`, runID)
	if err != nil {
		return Snapshot{}, err
	}
	for evidence.Next() {
		var item Evidence
		if err := evidence.Scan(&item.QuestionID, &item.DocID, &item.Probability); err != nil {
			evidence.Close()
			return Snapshot{}, err
		}
		snapshot.Evidence = append(snapshot.Evidence, item)
	}
	if err := evidence.Err(); err != nil {
		evidence.Close()
		return Snapshot{}, err
	}
	evidence.Close()

	gaps, err := tx.Query(ctx, `SELECT question_id, best_probability FROM coverage_gaps WHERE run_id=$1 ORDER BY question_id`, runID)
	if err != nil {
		return Snapshot{}, err
	}
	for gaps.Next() {
		var item CoverageGap
		if err := gaps.Scan(&item.QuestionID, &item.BestProbability); err != nil {
			gaps.Close()
			return Snapshot{}, err
		}
		snapshot.Gaps = append(snapshot.Gaps, item)
	}
	if err := gaps.Err(); err != nil {
		gaps.Close()
		return Snapshot{}, err
	}
	gaps.Close()

	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(id),0) FROM run_events WHERE run_id=$1`, runID).Scan(&snapshot.LastEventID); err != nil {
		return Snapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s *Store) EnsureRun(ctx context.Context, runID int64) error {
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM runs WHERE id=$1)`, runID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	return nil
}
