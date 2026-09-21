# Repository Operating Guide

## Scope

Keep this repository as a small demo. Make only changes required by the current task.

## Architecture

- `backend/app.py` contains FastAPI routes and request orchestration.
- `backend/db.py` contains only Qdrant operations.
- `backend/embeddings.py` contains only embedding calls.
- `backend/jev.py` contains only Jev calls and probability parsing.
- `backend/llm.py` contains only question and answer LLM calls.
- `tests/` contains backend tests.
- `web/` contains the Next.js frontend.
- Qdrant is the only database. Do not add PostgreSQL, SQL tables, migrations, an ORM, or persisted run state.
- The API returns one blocking response. Do not add SSE, polling, queues, workers, or background jobs.

Use the `ponytail` skill at its default `full` level for implementation, fixes, refactors, and reviews. Prefer the smallest correct change and do not add speculative abstractions or dependencies.

## Sources of truth

- `docs/planning/backend.md` defines backend behavior and the API response.
- `docs/planning/frontend.md` defines frontend behavior.
- Implementation and tests define current behavior once present.

## Commands

```sh
docker compose up -d --wait qdrant
python -m venv .venv
. .venv/bin/activate
pip install -r backend/requirements.txt
python -m unittest discover -s tests
uvicorn backend.app:app --reload --port 8080
cd web && npm run lint && npm test && npm run build
```

Do not report a command as passing unless it completed successfully.
