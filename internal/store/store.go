package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

type pgxTx = pgx.Tx

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{Pool: pool}, nil
}

func NewWithPool(pool *pgxpool.Pool) *Store { return &Store{Pool: pool} }

func (s *Store) Close() {
	if s != nil && s.Pool != nil {
		s.Pool.Close()
	}
}

type Run struct {
	ID                int64   `json:"id"`
	Query             string  `json:"query"`
	Status            string  `json:"status"`
	Answer            *string `json:"answer"`
	Error             *string `json:"error"`
	WarningCount      int     `json:"warning_count"`
	QuestionModel     string  `json:"-"`
	AnswerModel       string  `json:"-"`
	EmbeddingModel    string  `json:"-"`
	JevModel          string  `json:"-"`
	CoverageThreshold float32 `json:"-"`
}

type Question struct {
	ID      int64  `json:"id"`
	Ordinal int    `json:"ordinal"`
	Text    string `json:"text"`
}

type Document struct {
	DocID       string  `json:"doc_id"`
	Title       string  `json:"title"`
	Body        string  `json:"-"`
	TextPreview string  `json:"text_preview"`
	Rank        int     `json:"rank"`
	VectorScore float32 `json:"vector_score"`
}

type Judgment struct {
	QuestionID  int64    `json:"question_id"`
	DocID       string   `json:"doc_id"`
	Probability *float32 `json:"probability"`
	Error       *string  `json:"error"`
}

type Evidence struct {
	QuestionID  int64   `json:"question_id"`
	DocID       string  `json:"doc_id"`
	Probability float32 `json:"probability"`
}

type CoverageGap struct {
	QuestionID      int64   `json:"question_id"`
	BestProbability float32 `json:"best_probability"`
}

type Snapshot struct {
	LastEventID int64         `json:"last_event_id"`
	Run         Run           `json:"run"`
	Questions   []Question    `json:"questions"`
	Documents   []Document    `json:"documents"`
	Judgments   []Judgment    `json:"judgments"`
	Evidence    []Evidence    `json:"evidence"`
	Gaps        []CoverageGap `json:"gaps"`
}

type RunEvent struct {
	ID        int64           `json:"id"`
	RunID     int64           `json:"-"`
	EventType string          `json:"-"`
	Payload   json.RawMessage `json:"-"`
}

func appendEventTx(ctx context.Context, tx pgx.Tx, runID int64, eventType string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO run_events(run_id, event_type, payload) VALUES($1,$2,$3)`, runID, eventType, body)
	return err
}

func withTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetRun(ctx context.Context, id int64) (Run, error) {
	var run Run
	var answer, failure *string
	err := s.Pool.QueryRow(ctx, `
		SELECT id, query, status, answer, error, warning_count, question_model, answer_model, embedding_model, jev_model, coverage_threshold
		FROM runs WHERE id=$1`, id).Scan(&run.ID, &run.Query, &run.Status, &answer, &failure, &run.WarningCount, &run.QuestionModel, &run.AnswerModel, &run.EmbeddingModel, &run.JevModel, &run.CoverageThreshold)
	if err != nil {
		return Run{}, err
	}
	run.Answer, run.Error = answer, failure
	return run, nil
}

func (s *Store) RecoverInterrupted(ctx context.Context) error {
	return withTx(ctx, s.Pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `UPDATE runs SET status='failed', error='server interrupted while processing the run', updated_at=now(), completed_at=now() WHERE status NOT IN ('completed','failed') RETURNING id`)
		if err != nil {
			return err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, id := range ids {
			if err := appendEventTx(ctx, tx, id, "run.failed", map[string]string{"error": "server interrupted while processing the run"}); err != nil {
				return err
			}
		}
		return nil
	})
}
