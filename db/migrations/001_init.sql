CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS documents (
    doc_id text PRIMARY KEY,
    title text NOT NULL,
    body text NOT NULL,
    embedding vector NOT NULL,
    embedding_model text NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    query text NOT NULL,
    status text NOT NULL CHECK (status IN (
        'queued', 'generating_questions', 'retrieving', 'verifying',
        'selecting_evidence', 'generating_answer', 'completed', 'failed'
    )),
    answer text,
    error text,
    warning_count integer NOT NULL DEFAULT 0 CHECK (warning_count >= 0),
    question_model text,
    answer_model text,
    embedding_model text,
    jev_model text,
    coverage_threshold real NOT NULL CHECK (coverage_threshold >= 0 AND coverage_threshold <= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE TABLE IF NOT EXISTS questions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id bigint NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ordinal integer NOT NULL CHECK (ordinal >= 0),
    text text NOT NULL CHECK (length(btrim(text)) > 0),
    UNIQUE (run_id, ordinal),
    UNIQUE (run_id, id)
);

CREATE TABLE IF NOT EXISTS run_documents (
    run_id bigint NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    doc_id text NOT NULL REFERENCES documents(doc_id) ON DELETE CASCADE,
    rank integer NOT NULL CHECK (rank > 0),
    vector_score real NOT NULL CHECK (vector_score >= -1 AND vector_score <= 1),
    PRIMARY KEY (run_id, doc_id),
    UNIQUE (run_id, rank)
);

CREATE TABLE IF NOT EXISTS judgments (
    run_id bigint NOT NULL,
    question_id bigint NOT NULL,
    doc_id text NOT NULL,
    probability real CHECK (probability IS NULL OR (probability >= 0 AND probability <= 1)),
    error text,
    PRIMARY KEY (run_id, question_id, doc_id),
    FOREIGN KEY (run_id, question_id) REFERENCES questions(run_id, id) ON DELETE CASCADE,
    FOREIGN KEY (run_id, doc_id) REFERENCES run_documents(run_id, doc_id) ON DELETE CASCADE,
    CHECK (probability IS NOT NULL OR error IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS evidence (
    run_id bigint NOT NULL,
    question_id bigint NOT NULL,
    doc_id text NOT NULL,
    probability real NOT NULL CHECK (probability >= 0 AND probability <= 1),
    PRIMARY KEY (run_id, question_id),
    FOREIGN KEY (run_id, question_id, doc_id) REFERENCES judgments(run_id, question_id, doc_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS coverage_gaps (
    run_id bigint NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    question_id bigint NOT NULL,
    best_probability real NOT NULL CHECK (best_probability >= 0 AND best_probability <= 1),
    FOREIGN KEY (run_id, question_id) REFERENCES questions(run_id, id) ON DELETE CASCADE,
    PRIMARY KEY (run_id, question_id)
);

CREATE TABLE IF NOT EXISTS run_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id bigint NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS run_events_run_id_id_idx ON run_events(run_id, id);
CREATE INDEX IF NOT EXISTS run_documents_run_id_rank_idx ON run_documents(run_id, rank);
CREATE INDEX IF NOT EXISTS judgments_run_id_question_id_idx ON judgments(run_id, question_id);
