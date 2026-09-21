import os
from typing import Any

import httpx


HYBRID_COLLECTION = "multihoprag_benchmark"


def _headers() -> dict[str, str]:
    key = os.getenv("QDRANT_API_KEY")
    return {"api-key": key} if key else {}


def _collection_url() -> str:
    base = os.getenv("QDRANT_URL", "http://localhost:6333").rstrip("/")
    return f"{base}/collections/{os.getenv('QDRANT_COLLECTION', 'scifact')}"


def _hybrid_collection_url() -> str:
    base = os.getenv("QDRANT_URL", "http://localhost:6333").rstrip("/")
    return f"{base}/collections/{HYBRID_COLLECTION}"


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


def _normalize_points(points: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return [
        {
            "doc_id": str((point.get("payload") or {}).get("doc_id", point["id"])),
            "title": str((point.get("payload") or {}).get("title", "Untitled")),
            "text": str(
                (point.get("payload") or {}).get(
                    "text", (point.get("payload") or {}).get("body", "")
                )
            ),
            "rank": rank,
            "vector_score": float(point["score"]),
        }
        for rank, point in enumerate(points, 1)
    ]


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
    return _normalize_points(response.json().get("result", {}).get("points", []))


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


async def ensure_hybrid_collection(vector_size: int, client: httpx.AsyncClient) -> None:
    url = _hybrid_collection_url()
    response = await client.get(url, headers=_headers())
    if response.status_code == 200:
        params = response.json().get("result", {}).get("config", {}).get("params", {})
        vectors = params.get("vectors", {})
        sparse_vectors = params.get("sparse_vectors", {})
        dense = vectors.get("dense", {}) if isinstance(vectors, dict) else {}
        if (
            not isinstance(vectors, dict)
            or not isinstance(sparse_vectors, dict)
            or not isinstance(dense, dict)
            or set(vectors) != {"dense"}
            or set(sparse_vectors) != {"bm25"}
            or dense.get("size") != vector_size
            or dense.get("distance") != "Cosine"
        ):
            raise ValueError(f"Incompatible Qdrant schema for {HYBRID_COLLECTION}")
        return
    if response.status_code != 404:
        response.raise_for_status()

    response = await client.put(
        url,
        headers=_headers(),
        json={
            "vectors": {"dense": {"size": vector_size, "distance": "Cosine"}},
            "sparse_vectors": {"bm25": {}},
        },
    )
    response.raise_for_status()


async def upsert_hybrid(
    documents: list[dict[str, Any]], client: httpx.AsyncClient
) -> None:
    points = []
    for document in documents:
        sparse = document.get("sparse_vector")
        if sparse is None:
            sparse = document.get("bm25", document.get("sparse"))
        if sparse is None:
            sparse = {
                "indices": document.get(
                    "sparse_indices", document.get("bm25_indices")
                ),
                "values": document.get("sparse_values", document.get("bm25_values")),
            }
        points.append(
            {
                "id": document.get("point_id", document.get("id")),
                "vector": {
                    "dense": document.get(
                        "vector", document.get("dense_vector", document.get("dense"))
                    ),
                    "bm25": sparse,
                },
                "payload": {
                    "doc_id": str(document["doc_id"]),
                    "title": document.get("title", "Untitled"),
                    "text": document.get("text", document.get("body", "")),
                },
            }
        )

    response = await client.put(
        f"{_hybrid_collection_url()}/points",
        headers=_headers(),
        params={"wait": "true"},
        json={"points": points},
    )
    response.raise_for_status()


async def hybrid_search(
    dense_vector: list[float],
    sparse_vector: dict[str, Any],
    candidate_limit: int,
    result_limit: int,
    client: httpx.AsyncClient,
) -> list[dict[str, Any]]:
    response = await client.post(
        f"{_hybrid_collection_url()}/points/query",
        headers=_headers(),
        json={
            "prefetch": [
                {"query": dense_vector, "using": "dense", "limit": candidate_limit},
                {"query": sparse_vector, "using": "bm25", "limit": candidate_limit},
            ],
            "query": {"fusion": "rrf"},
            "limit": result_limit,
            "with_payload": True,
            "with_vector": False,
        },
    )
    response.raise_for_status()
    return _normalize_points(response.json().get("result", {}).get("points", []))


async def existing_point_ids(
    ids: list[str], client: httpx.AsyncClient
) -> set[str]:
    response = await client.post(
        f"{_hybrid_collection_url()}/points",
        headers=_headers(),
        json={"ids": ids, "with_payload": False, "with_vector": False},
    )
    response.raise_for_status()
    return {
        point["id"]
        for point in response.json().get("result", [])
        if "id" in point
    }
