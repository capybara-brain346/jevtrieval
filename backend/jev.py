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
    documents: list[dict[str, Any]],
    query: str,
    questions: list[str],
    client: httpx.AsyncClient,
) -> list[dict[str, Any]]:
    semaphore = asyncio.Semaphore(int(os.getenv("JEV_CONCURRENCY", "8")))
    relevance_question = (
        "Does this document contain substantive evidence useful for answering "
        f"the user's query: {query}"
    )
    scoring_questions = [relevance_question, *questions]

    async def score(document: dict[str, Any]) -> dict[str, Any]:
        try:
            async with semaphore:
                probabilities = await _score(document, scoring_questions, client)
        except (httpx.HTTPError, KeyError, TypeError, ValueError) as error:
            probabilities = [
                {"question": question, "probability": None, "error": str(error)}
                for question in scoring_questions
            ]
        relevance, *helper_probabilities = probabilities
        return {
            **document,
            "query_probability": relevance["probability"],
            "query_error": relevance["error"],
            "probabilities": helper_probabilities,
        }

    return list(await asyncio.gather(*(score(document) for document in documents)))


def select_documents(
    documents: list[dict], limit: int = 5
) -> list[dict]:
    if not documents:
        return []

    coverage = [0.0] * len(documents[0]["probabilities"])
    remaining = documents.copy()
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
