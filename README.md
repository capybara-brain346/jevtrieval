# Jevtrieval

Jevtrieval is a retrieval-augmented generation demo. It retrieves documents from Qdrant, scores their relevance with Jev, and generates an answer from the selected documents.

The repository includes a FastAPI backend, a Next.js frontend, and an offline MultiHopRAG benchmark.

## How it works

For each search request, the backend:

1. Generates supporting questions from the query.
2. Embeds the query.
3. Retrieves 25 vector candidates from Qdrant.
4. Scores each candidate against the query and supporting questions with Jev.
5. Selects up to five documents.
6. Generates an answer from the selected documents.

A document must have a direct-query relevance score of at least `0.5`. Selection uses 70% direct relevance and 30% supporting-question coverage.

Qdrant stores document vectors and payloads. Request data is not persisted. The API returns one complete JSON response.

## Prerequisites

Install the following tools:

- Docker
- Python 3.12 or later
- [uv](https://docs.astral.sh/uv/)
- Node.js and npm

An OpenRouter API key is required.

## Run the backend

1. Create the environment file:

   ```sh
   cp .env.example .env
   ```

2. Set `OPENROUTER_API_KEY` in `.env`.

3. Start Qdrant and install the Python dependencies:

   ```sh
   docker compose up -d --wait qdrant
   uv sync
   ```

4. Download the SciFact dataset:

   ```sh
   scripts/download-scifact.sh
   ```

5. Start the API:

   ```sh
   uv run --env-file .env uvicorn backend.app:app --reload --port 8080
   ```

6. Index SciFact once:

   ```sh
   curl -X POST http://localhost:8080/v1/index
   ```

The index operation skips embedding when Qdrant already contains all 5,183 SciFact documents.

## Run the frontend

```sh
cd web
cp .env.example .env.local
npm install
npm run dev
```

Open `http://localhost:3000`.

## API

### Check service health

```http
GET /healthz
```

### Index SciFact

```http
POST /v1/index
```

### Search documents

```http
POST /v1/search
Content-Type: application/json

{"query":"What evidence supports the claim?"}
```

The response contains the original query, generated supporting questions, selected documents, and generated answer.

Interactive API documentation is available at `http://localhost:8080/docs` while the backend is running.

## Run tests

```sh
uv run python -m unittest discover -s tests
```

For the frontend:

```sh
cd web
npm run lint
npm test
npm run build
```

## Run the MultiHopRAG benchmark

The benchmark compares Jev retrieval with dense and BM25 retrieval combined by reciprocal rank fusion. It reports Recall@5, AllEvidence@5, MRR@5, and nDCG@5.

1. Install the Hugging Face CLI and authenticate if required.

2. Download the dataset:

   ```sh
   scripts/download-multihoprag.sh
   ```

3. Index the corpus:

   ```sh
   uv run --env-file .env python scripts/benchmark-multihoprag.py index
   ```

4. Run a benchmark sample:

   ```sh
   uv run --env-file .env python scripts/benchmark-multihoprag.py run \
     --limit 10 \
     --output data/multihoprag/results/validation.json \
     --records data/multihoprag/results/validation.jsonl
   ```

`--limit` is required. Use `--offset`, `--question-type`, or `--seed` to select a different sample.

Indexing validates the corpus before embedding. Existing UUID5 points are skipped. The command verifies the final Qdrant point count before it exits.
