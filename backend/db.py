import os
from typing import Any

import httpx


def _headers() -> dict[str, str]:
    key = os.getenv("QDRANT_API_KEY")
    return {"api-key": key} if key else {}


def _collection_url() -> str:
    base = os.getenv("QDRANT_URL", "http://localhost:6333").rstrip("/")
    return f"{base}/collections/{os.getenv('QDRANT_COLLECTION', 'scifact')}"


async def health(client: httpx.AsyncClient) -> bool:
    try:
        response = await client.get(
            f"{os.getenv('QDRANT_URL', 'http://localhost:6333').rstrip('/')}/readyz",
            headers=_headers(),
        )
        return response.is_success
    except httpx.HTTPError:
        return False


async def ensure_collection(vector_size: int, client: httpx.AsyncClient) -> None:
    response = await client.get(_collection_url(), headers=_headers())
    if response.status_code == 200:
        return
    if response.status_code != 404:
        response.raise_for_status()
    response = await client.put(
        _collection_url(),
        headers=_headers(),
        json={"vectors": {"size": vector_size, "distance": "Cosine"}},
    )
    response.raise_for_status()


async def count(client: httpx.AsyncClient) -> int:
    response = await client.post(
        f"{_collection_url()}/points/count", headers=_headers(), json={"exact": True}
    )
    if response.status_code == 404:
        return 0
    response.raise_for_status()
    return int(response.json()["result"]["count"])


async def search(
    vector: list[float], limit: int, client: httpx.AsyncClient
) -> list[dict[str, Any]]:
    response = await client.post(
        f"{_collection_url()}/points/query",
        headers=_headers(),
        json={
            "query": vector,
            "limit": limit,
            "with_payload": True,
            "with_vector": False,
        },
    )
    response.raise_for_status()
    points = response.json().get("result", {}).get("points", [])
    documents = []
    for rank, point in enumerate(points, 1):
        payload = point.get("payload") or {}
        documents.append(
            {
                "doc_id": str(payload.get("doc_id", point["id"])),
                "title": str(payload.get("title", "Untitled")),
                "text": str(payload.get("text", payload.get("body", ""))),
                "rank": rank,
                "vector_score": float(point["score"]),
            }
        )
    return documents


async def upsert(documents: list[dict[str, Any]], client: httpx.AsyncClient) -> None:
    points = [
        {
            "id": int(document["id"])
            if str(document["id"]).isdigit()
            else document["id"],
            "vector": document["vector"],
            "payload": {
                "doc_id": str(document["id"]),
                "title": document.get("title", "Untitled"),
                "text": document.get("text", ""),
            },
        }
        for document in documents
    ]
    response = await client.put(
        f"{_collection_url()}/points",
        headers=_headers(),
        params={"wait": "true"},
        json={"points": points},
    )
    response.raise_for_status()
