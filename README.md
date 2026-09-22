# jevtrieval

A small FastAPI demo that retrieves documents from Qdrant, scores them with Jev, and generates an evidence-grounded answer.

## Architecture

- Qdrant stores only document vectors and payloads.
- Requests, questions, probabilities, and answers live only for the duration of one HTTP request.
- `POST /v1/search` returns one complete JSON response. There are no SQL tables, run records, background jobs, or event streams.

The included SciFact JSONL file can be embedded into Qdrant once through `POST /v1/index`. The route is idempotent after all 5,183 documents are present; application startup never performs an expensive hidden import.

## Run

```sh
cp .env.example .env
# Add OPENROUTER_API_KEY to .env.

docker compose up -d --wait qdrant
uv sync
uv run python -m unittest discover -s tests
uv run --env-file .env uvicorn backend.app:app --reload --port 8080

# Run once to embed data/scifact/docs.jsonl into Qdrant.
curl -X POST http://localhost:8080/v1/index
```

Start the frontend separately:

```sh
cd web
cp .env.example .env.local
npm install
npm run dev
```

## API

```text
GET  /healthz
POST /v1/index
POST /v1/search  {"query":"..."}
```

The search response contains the generated questions and up to five documents selected from 25 vector candidates. Documents need at least 0.5 direct-query relevance; eligible documents are selected using 70% direct relevance and 30% question-coverage gain.

## MultiHopRAG benchmark

The offline benchmark imports backend modules directly; it does not add benchmark API routes. Download or verify both dataset files, start Qdrant, then index the corpus:

```sh
scripts/download-multihoprag.sh
uv run --env-file .env python scripts/benchmark-multihoprag.py index
uv run --env-file .env python scripts/benchmark-multihoprag.py run \
  --limit 10 \
  --output data/multihoprag/results/validation.json \
  --records data/multihoprag/results/validation.jsonl
```

Indexing validates the corpus before the first embedding request, resumes by skipping existing UUID5 points, reports progress, and verifies the final Qdrant count. It compares Jev retrieval with dense-plus-BM25 RRF retrieval using Recall@5, AllEvidence@5, MRR@5, and nDCG@5. `--limit` is required for benchmark runs; use `--offset`, `--question-type`, and `--seed` to select another reproducible slice.
