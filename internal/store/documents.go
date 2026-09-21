package store

import (
	"context"
	"fmt"
	"math"
	"strings"
)

func VectorLiteral(values []float64) (string, error) {
	if len(values) == 0 {
		return "", fmt.Errorf("embedding vector is empty")
	}
	var builder strings.Builder
	builder.WriteByte('[')
	for i, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return "", fmt.Errorf("embedding vector contains a non-finite value")
		}
		if i > 0 {
			builder.WriteByte(',')
		}
		fmt.Fprintf(&builder, "%g", value)
	}
	builder.WriteByte(']')
	return builder.String(), nil
}

func (s *Store) DocumentModels(ctx context.Context, ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT doc_id, embedding_model FROM documents WHERE doc_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models := make(map[string]string, len(ids))
	for rows.Next() {
		var id, model string
		if err := rows.Scan(&id, &model); err != nil {
			return nil, err
		}
		models[id] = model
	}
	return models, rows.Err()
}

func (s *Store) UpsertDocuments(ctx context.Context, documents []Document, embeddings [][]float64, model string, replace bool) error {
	if len(documents) != len(embeddings) {
		return fmt.Errorf("documents and embeddings have different lengths")
	}
	if len(documents) == 0 {
		return nil
	}
	dimension := len(embeddings[0])
	if dimension == 0 {
		return fmt.Errorf("embedding vector is empty")
	}
	for _, embedding := range embeddings {
		if len(embedding) != dimension {
			return fmt.Errorf("embedding dimensions are inconsistent")
		}
	}
	return withTx(ctx, s.Pool, func(tx pgxTx) error {
		var minimum, maximum int
		if err := tx.QueryRow(ctx, `SELECT COALESCE(MIN(vector_dims(embedding)),0), COALESCE(MAX(vector_dims(embedding)),0) FROM documents WHERE embedding_model=$1`, model).Scan(&minimum, &maximum); err != nil {
			return err
		}
		if (minimum != 0 && minimum != dimension) || (maximum != 0 && maximum != dimension) {
			return fmt.Errorf("embedding dimension %d does not match existing model dimension %d", dimension, minimum)
		}
		for i, document := range documents {
			vector, err := VectorLiteral(embeddings[i])
			if err != nil {
				return err
			}
			if replace {
				_, err = tx.Exec(ctx, `INSERT INTO documents(doc_id, title, body, embedding, embedding_model) VALUES($1,$2,$3,$4::vector,$5)
					ON CONFLICT(doc_id) DO UPDATE SET title=EXCLUDED.title, body=EXCLUDED.body, embedding=EXCLUDED.embedding, embedding_model=EXCLUDED.embedding_model`,
					document.DocID, document.Title, document.Body, vector, model)
			} else {
				_, err = tx.Exec(ctx, `INSERT INTO documents(doc_id, title, body, embedding, embedding_model) VALUES($1,$2,$3,$4::vector,$5)
					ON CONFLICT(doc_id) DO UPDATE SET title=EXCLUDED.title, body=EXCLUDED.body, embedding=EXCLUDED.embedding, embedding_model=EXCLUDED.embedding_model
					WHERE documents.embedding_model=EXCLUDED.embedding_model`, document.DocID, document.Title, document.Body, vector, model)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) SearchDocuments(ctx context.Context, embedding []float64, model string, limit int) ([]Document, error) {
	vector, err := VectorLiteral(embedding)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT doc_id, title, body, 1 - (embedding <=> $1::vector) AS vector_score
		FROM documents WHERE embedding_model=$2
		ORDER BY embedding <=> $1::vector LIMIT $3`, vector, model, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var documents []Document
	rank := 1
	for rows.Next() {
		var document Document
		if err := rows.Scan(&document.DocID, &document.Title, &document.Body, &document.VectorScore); err != nil {
			return nil, err
		}
		document.Rank = rank
		document.TextPreview = preview(document.Body)
		rank++
		documents = append(documents, document)
	}
	return documents, rows.Err()
}

func preview(body string) string {
	body = strings.TrimSpace(body)
	if len([]rune(body)) <= 280 {
		return body
	}
	return string([]rune(body)[:280]) + "…"
}

func (s *Store) SaveRetrievedDocuments(ctx context.Context, runID int64, documents []Document) error {
	return withTx(ctx, s.Pool, func(tx pgxTx) error {
		for _, document := range documents {
			if _, err := tx.Exec(ctx, `INSERT INTO run_documents(run_id, doc_id, rank, vector_score) VALUES($1,$2,$3,$4)`, runID, document.DocID, document.Rank, document.VectorScore); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE runs SET updated_at=now() WHERE id=$1`, runID); err != nil {
			return err
		}
		payload := make([]Document, len(documents))
		for i, document := range documents {
			payload[i] = document
		}
		return appendEventTx(ctx, tx, runID, "documents.retrieved", map[string]any{"documents": payload})
	})
}
