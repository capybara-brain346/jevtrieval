import json
import logging
import os
from contextlib import asynccontextmanager
from pathlib import Path

import httpx
from fastapi import FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from backend import db, embeddings, jev, llm

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class SearchRequest(BaseModel):
    query: str = Field(min_length=1, max_length=2_000)


def _select_documents(
    documents: list[dict], limit: int = 5
) -> list[dict]:
    if not documents:
        return []

    coverage = [0.0] * len(documents[0]["probabilities"])
    remaining = [
        document
        for document in documents
        if (document["query_probability"] or 0.0) >= 0.5
    ]
    selected = []
    while remaining and len(selected) < limit:
        def score(document: dict) -> float:
            gain = sum(
                max(0.0, (item["probability"] or 0.0) - coverage[index])
                for index, item in enumerate(document["probabilities"])
            ) / len(coverage)
            return 0.7 * (document["query_probability"] or 0.0) + 0.3 * gain

        document = max(
            remaining, key=lambda candidate: (score(candidate), candidate["vector_score"])
        )
        selected.append(document)
        coverage = [
            max(current, item["probability"] or 0.0)
            for current, item in zip(coverage, document["probabilities"])
        ]
        remaining.remove(document)
    return selected


@asynccontextmanager
async def lifespan(application: FastAPI):
    application.state.http = httpx.AsyncClient(timeout=httpx.Timeout(120, connect=10))
    try:
        yield
    finally:
        await application.state.http.aclose()


app = FastAPI(title="Jev Retrieval", lifespan=lifespan)
app.add_middleware(
    CORSMiddleware,
    allow_origins=[os.getenv("ALLOWED_ORIGIN", "http://localhost:3000")],
    allow_methods=["GET", "POST", "OPTIONS"],
    allow_headers=["Content-Type"],
)


@app.get("/healthz")
async def healthz(request: Request):
    qdrant = await db.health(request.app.state.http)
    return {"status": "ok" if qdrant else "degraded", "qdrant": qdrant}


@app.post("/v1/index")
async def index(request: Request):
    path = Path(os.getenv("SCIFACT_PATH", "data/scifact/docs.jsonl"))
    try:
        documents = [
            json.loads(line) for line in path.read_text().splitlines() if line.strip()
        ]
        client = request.app.state.http
        if await db.count(client) == len(documents):
            return {"indexed": len(documents), "skipped": True}
        batch_size = int(os.getenv("EMBEDDING_BATCH_SIZE", "64"))
        for start in range(0, len(documents), batch_size):
            batch = documents[start : start + batch_size]
            vectors = await embeddings.embed_many(
                [document["text"] for document in batch], client
            )
            if start == 0:
                await db.ensure_collection(len(vectors[0]), client)
            await db.upsert(
                [
                    {**document, "id": document["doc_id"], "vector": vector}
                    for document, vector in zip(batch, vectors)
                ],
                client,
            )
        return {"indexed": len(documents), "skipped": False}
    except (
        OSError,
        json.JSONDecodeError,
        httpx.HTTPError,
        KeyError,
        TypeError,
        ValueError,
    ) as error:
        logger.exception("Indexing failed: %s", error)
        raise HTTPException(status_code=502, detail="Indexing failed") from error


@app.post("/v1/search")
async def search(body: SearchRequest, request: Request):
    query = body.query.strip()
    if not query:
        raise HTTPException(status_code=422, detail="Query must not be blank")

    client = request.app.state.http
    try:
        questions = await llm.generate_questions(query, client)
        vector = await embeddings.embed(query, client)
        documents = await db.search(
            vector, int(os.getenv("RETRIEVAL_LIMIT", "25")), client
        )
        documents = await jev.score_documents(documents, query, questions, client)
        documents = _select_documents(documents)
        answer = await llm.generate_answer(query, documents, client)
        return {
            "query": query,
            "questions": questions,
            "documents": documents,
            "answer": answer,
        }
    except (httpx.HTTPError, KeyError, TypeError, ValueError) as error:
        logger.exception("Search failed: %s", error)
        raise HTTPException(status_code=502, detail="Search pipeline failed") from error
