import os

import httpx


async def embed_many(texts: list[str], client: httpx.AsyncClient) -> list[list[float]]:
    response = await client.post(
        f"{os.getenv('OPENROUTER_BASE_URL', 'https://openrouter.ai/api').rstrip('/')}/v1/embeddings",
        headers={"Authorization": f"Bearer {os.environ['OPENROUTER_API_KEY']}"},
        json={"model": os.environ["OPENROUTER_EMBEDDING_MODEL"], "input": texts},
    )
    response.raise_for_status()
    data = sorted(response.json()["data"], key=lambda item: item.get("index", 0))
    vectors = [item.get("embedding") for item in data]
    if len(vectors) != len(texts) or any(
        not vector or not all(isinstance(value, (int, float)) for value in vector)
        for vector in vectors
    ):
        raise ValueError("Embedding provider returned invalid vectors")
    return [[float(value) for value in vector] for vector in vectors]


async def embed(text: str, client: httpx.AsyncClient) -> list[float]:
    return (await embed_many([text], client))[0]
