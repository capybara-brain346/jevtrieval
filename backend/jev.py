import asyncio
import os
from typing import Any

import httpx

_RETRY_STATUSES = {429, 502, 503, 504}


def _headers() -> dict[str, str]:
    return {"Authorization": f"Bearer {os.environ['OPENROUTER_API_KEY']}"}


def _probabilities(data: dict[str, Any], questions: list[str]) -> list[dict[str, Any]]:
    answers = data.get("answers", {})
    results = []
    for index, question in enumerate(questions):
        answer = answers.get(f"q_{index}")
        probability = (
            answer.get("noul")
            if isinstance(answer, dict) and answer.get("type") == "noul"
            else None
        )
        if not isinstance(probability, (int, float)) or not 0 <= probability <= 1:
            raise ValueError(f"Jev returned an invalid answer for q_{index}")
        results.append(
            {"question": question, "probability": float(probability), "error": None}
        )
    return results


async def _score(
    document: dict[str, Any], questions: list[str], client: httpx.AsyncClient
) -> list[dict[str, Any]]:
    payload = {
        "model": os.getenv("OPENROUTER_JEV_MODEL", "~typesafe/jev-latest"),
        "state": {
            "doc_id": document["doc_id"],
            "title": document["title"],
            "text": document["text"],
        },
        "questions": {
            f"q_{index}": {
                "type": "noul",
                "instructions": question,
                "criteria": {
                    "true": "Substantive evidence relevant to the question is present.",
                    "false": "The evidence is absent or not useful.",
                },
            }
            for index, question in enumerate(questions)
        },
    }
    url = f"{os.getenv('OPENROUTER_BASE_URL', 'https://openrouter.ai/api').rstrip('/')}/alpha/decisions"
    for attempt in range(3):
        response = await client.post(url, headers=_headers(), json=payload)
        if response.status_code not in _RETRY_STATUSES or attempt == 2:
            response.raise_for_status()
            return _probabilities(response.json(), questions)
        await asyncio.sleep(0.25 * (attempt + 1))
    raise AssertionError("unreachable")


async def score_documents(
    documents: list[dict[str, Any]], questions: list[str], client: httpx.AsyncClient
) -> list[dict[str, Any]]:
    semaphore = asyncio.Semaphore(int(os.getenv("JEV_CONCURRENCY", "8")))

    async def score(document: dict[str, Any]) -> dict[str, Any]:
        try:
            async with semaphore:
                probabilities = await _score(document, questions, client)
        except (httpx.HTTPError, KeyError, TypeError, ValueError) as error:
            probabilities = [
                {"question": question, "probability": None, "error": str(error)}
                for question in questions
            ]
        return {**document, "probabilities": probabilities}

    return list(await asyncio.gather(*(score(document) for document in documents)))
