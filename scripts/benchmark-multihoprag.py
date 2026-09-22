#!/usr/bin/env python3
"""Index and benchmark the MultiHopRAG retrieval pipelines."""

from __future__ import annotations

import argparse
import asyncio
import hashlib
import json
import math
import os
import random
import re
import subprocess
import sys
import uuid
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from statistics import fmean
from typing import Any

import httpx
import tiktoken

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from backend import db, embeddings, jev, llm  # noqa: E402

CORPUS_PATH = ROOT / "data/multihoprag/corpus.json"
EXAMPLES_PATH = ROOT / "data/multihoprag/MultiHopRAG.json"
RETRIEVAL_LIMIT = 25
RESULT_LIMIT = 5
BM25_K1 = 1.2
BM25_B = 0.75
EMBEDDING_TOKEN_LIMIT = 8192
TOKEN_RE = re.compile(r"\w+", re.UNICODE)
EMBEDDING_ENCODING = tiktoken.get_encoding("cl100k_base")
METRICS = ("recall_at_5", "all_evidence_at_5", "mrr_at_5", "ndcg_at_5")


def load_json(path: Path) -> list[dict[str, Any]]:
    value = json.loads(path.read_text())
    if not isinstance(value, list):
        raise ValueError(f"Expected a JSON array in {path}")
    return value


def point_id_for_url(url: str) -> str:
    return str(uuid.uuid5(uuid.NAMESPACE_URL, url))


def normalize_corpus(records: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Validate and normalize corpus records before any embedding request."""
    documents = []
    seen_urls = set()
    for index, record in enumerate(records):
        if not isinstance(record, dict):
            raise ValueError(f"Corpus record {index} is not an object")
        url = record.get("url")
        title = record.get("title")
        body = record.get("body")
        if not isinstance(url, str) or not url.strip():
            raise ValueError(f"Corpus record {index} has no URL")
        if not isinstance(title, str) or not isinstance(body, str):
            raise ValueError(f"Corpus record {index} has invalid title or body")
        if url in seen_urls:
            raise ValueError(f"Duplicate corpus URL: {url}")
        seen_urls.add(url)
        documents.append(
            {
                "doc_id": url,
                "point_id": point_id_for_url(url),
                "title": title,
                "text": f"{title}\n\n{body}",
            }
        )
    if not documents:
        raise ValueError("Corpus is empty")
    return documents


def tokenize(text: str) -> list[str]:
    return TOKEN_RE.findall(text.casefold())


def embedding_text(text: str) -> str:
    tokens = EMBEDDING_ENCODING.encode(text)
    return text if len(tokens) <= EMBEDDING_TOKEN_LIMIT else EMBEDDING_ENCODING.decode(tokens[:EMBEDDING_TOKEN_LIMIT])


def bm25_statistics(documents: list[dict[str, Any]]) -> dict[str, Any]:
    token_lists = [tokenize(document["text"]) for document in documents]
    document_frequencies = Counter(
        token for tokens in token_lists for token in set(tokens)
    )
    vocabulary = {
        token: index
        for index, token in enumerate(sorted(document_frequencies))
    }
    return {
        "vocabulary": vocabulary,
        "document_frequencies": dict(document_frequencies),
        "document_count": len(documents),
        "average_length": sum(map(len, token_lists)) / len(token_lists),
    }


def _idf(document_count: int, document_frequency: int) -> float:
    return math.log1p(
        (document_count - document_frequency + 0.5)
        / (document_frequency + 0.5)
    )


def bm25_document_vector(
    tokens: list[str],
    vocabulary: dict[str, int],
    document_frequencies: dict[str, int],
    document_count: int,
    average_length: float,
    k1: float = BM25_K1,
    b: float = BM25_B,
) -> dict[str, list[float] | list[int]]:
    counts = Counter(tokens)
    length = len(tokens)
    values = []
    for token, term_frequency in counts.items():
        if token not in vocabulary:
            continue
        denominator = term_frequency + k1 * (
            1 - b + b * length / (average_length or 1.0)
        )
        weight = _idf(document_count, document_frequencies[token]) * (
            term_frequency * (k1 + 1) / denominator
        )
        values.append((vocabulary[token], weight))
    values.sort()
    return {
        "indices": [index for index, _ in values],
        "values": [float(weight) for _, weight in values],
    }


def bm25_query_vector(
    query: str, vocabulary: dict[str, int]
) -> dict[str, list[float] | list[int]]:
    indices = sorted({vocabulary[token] for token in tokenize(query) if token in vocabulary})
    return {"indices": indices, "values": [1.0] * len(indices)}


def add_bm25_vectors(
    documents: list[dict[str, Any]], stats: dict[str, Any]
) -> list[dict[str, Any]]:
    return [
        {
            **document,
            "sparse_vector": bm25_document_vector(
                tokenize(document["text"]),
                stats["vocabulary"],
                stats["document_frequencies"],
                stats["document_count"],
                stats["average_length"],
            ),
        }
        for document in documents
    ]


def prepare_corpus(path: Path = CORPUS_PATH) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    documents = normalize_corpus(load_json(path))
    stats = bm25_statistics(documents)
    return add_bm25_vectors(documents, stats), stats


async def index_documents(
    documents: list[dict[str, Any]], client: httpx.AsyncClient, batch_size: int
) -> int:
    if batch_size < 1:
        raise ValueError("Embedding batch size must be positive")

    collection_ready = False
    total = len(documents)
    for start in range(0, total, batch_size):
        batch = documents[start : start + batch_size]
        existing = await db.existing_point_ids(
            [document["point_id"] for document in batch], client
        )
        missing = [
            document for document in batch if document["point_id"] not in existing
        ]
        if missing:
            vectors = await embeddings.embed_many(
                [embedding_text(document["text"]) for document in missing], client
            )
            if (
                len(vectors) != len(missing)
                or not vectors
                or any(not vector for vector in vectors)
                or len({len(vector) for vector in vectors}) != 1
            ):
                raise ValueError("Embedding provider returned invalid vectors")
            if not collection_ready:
                await db.ensure_hybrid_collection(len(vectors[0]), client)
                collection_ready = True
            await db.upsert_hybrid(
                [
                    {**document, "vector": vector}
                    for document, vector in zip(missing, vectors)
                ],
                client,
            )
        print(f"Indexed {min(start + len(batch), total)}/{total}")

    count = await db.hybrid_count(client)
    if count != total:
        raise RuntimeError(f"Expected {total} benchmark points, found {count}")
    return count


def extract_evidence_urls(example: dict[str, Any]) -> list[str]:
    evidence = example.get("evidence_list")
    if not isinstance(evidence, list) or not evidence:
        raise ValueError("Example has no evidence_list")
    urls = []
    for item in evidence:
        if not isinstance(item, dict) or not isinstance(item.get("url"), str):
            raise ValueError("Example contains invalid evidence")
        if item["url"] not in urls:
            urls.append(item["url"])
    return urls


def recall_at_5(retrieved: list[str], relevant: set[str] | list[str]) -> float:
    relevant = set(relevant)
    return len(set(retrieved[:RESULT_LIMIT]) & relevant) / len(relevant) if relevant else 0.0


def all_evidence_at_5(
    retrieved: list[str], relevant: set[str] | list[str]
) -> float:
    relevant = set(relevant)
    return float(bool(relevant) and relevant.issubset(retrieved[:RESULT_LIMIT]))


def mrr_at_5(retrieved: list[str], relevant: set[str] | list[str]) -> float:
    relevant = set(relevant)
    for rank, url in enumerate(retrieved[:RESULT_LIMIT], 1):
        if url in relevant:
            return 1.0 / rank
    return 0.0


def ndcg_at_5(retrieved: list[str], relevant: set[str] | list[str]) -> float:
    relevant = set(relevant)
    if not relevant:
        return 0.0
    dcg = sum(
        1.0 / math.log2(rank + 1)
        for rank, url in enumerate(retrieved[:RESULT_LIMIT], 1)
        if url in relevant
    )
    ideal = sum(
        1.0 / math.log2(rank + 1)
        for rank in range(1, min(len(relevant), RESULT_LIMIT) + 1)
    )
    return dcg / ideal


def score_retrieval(
    retrieved: list[str], relevant: list[str]
) -> dict[str, float]:
    return {
        "recall_at_5": recall_at_5(retrieved, relevant),
        "all_evidence_at_5": all_evidence_at_5(retrieved, relevant),
        "mrr_at_5": mrr_at_5(retrieved, relevant),
        "ndcg_at_5": ndcg_at_5(retrieved, relevant),
    }


def select_examples(
    examples: list[dict[str, Any]],
    offset: int,
    limit: int,
    question_type: str | None = None,
    seed: int | None = None,
) -> list[tuple[int, dict[str, Any]]]:
    if offset < 0 or limit < 1:
        raise ValueError("Offset must be non-negative and limit must be positive")
    selected = [
        (index, example)
        for index, example in enumerate(examples)
        if question_type is None or example.get("question_type") == question_type
    ]
    if seed is not None:
        random.Random(seed).shuffle(selected)
    return selected[offset : offset + limit]


def method_order(query_index: int) -> tuple[str, str]:
    return ("jev", "rag") if query_index % 2 == 0 else ("rag", "jev")


async def benchmark_example(
    query_index: int,
    example: dict[str, Any],
    vocabulary: dict[str, int],
    client: httpx.AsyncClient,
) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    query = example.get("query")
    if not isinstance(query, str) or not query.strip():
        raise ValueError(f"Example {query_index} has no query")
    expected = extract_evidence_urls(example)
    dense_vector = None
    questions = None

    async def get_dense() -> list[float]:
        nonlocal dense_vector
        if dense_vector is None:
            dense_vector = await embeddings.embed(query, client)
        return dense_vector

    async def get_questions() -> list[str]:
        nonlocal questions
        if questions is None:
            questions = await llm.generate_questions(query, client)
        return questions

    async def run_jev() -> list[str]:
        query_questions = await get_questions()
        query_vector = await get_dense()
        documents = await db.hybrid_dense_search(
            query_vector, RETRIEVAL_LIMIT, client
        )
        documents = await jev.score_documents(
            documents, query, query_questions, client
        )
        documents = jev.select_documents(documents, RESULT_LIMIT)
        return [str(document["doc_id"]) for document in documents]

    async def run_rag() -> list[str]:
        documents = await db.hybrid_search(
            await get_dense(),
            bm25_query_vector(query, vocabulary),
            RETRIEVAL_LIMIT,
            RESULT_LIMIT,
            client,
        )
        return [str(document["doc_id"]) for document in documents]

    runners = {"jev": run_jev, "rag": run_rag}
    results = {}
    failures = []
    for method in method_order(query_index):
        try:
            retrieved = await runners[method]()
            results[method] = {
                "retrieved_urls": retrieved,
                **score_retrieval(retrieved, expected),
                "error": None,
            }
        except Exception as error:
            message = f"{type(error).__name__}: {error}"
            results[method] = {
                "retrieved_urls": [],
                **{metric: None for metric in METRICS},
                "error": message,
            }
            failures.append({"query_index": query_index, "method": method, "error": message})

    return (
        {
            "query_index": query_index,
            "query": query,
            "question_type": example.get("question_type"),
            "expected_evidence_urls": expected,
            "jev": results["jev"],
            "rag": results["rag"],
        },
        failures,
    )


def aggregate_records(records: list[dict[str, Any]]) -> dict[str, dict[str, Any]]:
    aggregate = {}
    for method in ("jev", "rag"):
        successful = [record[method] for record in records if record[method]["error"] is None]
        aggregate[method] = {
            "attempted": len(records),
            "successful": len(successful),
            **{
                metric: fmean(result[metric] for result in successful)
                if successful
                else None
                for metric in METRICS
            },
        }
    return aggregate


def dataset_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def git_sha() -> str:
    try:
        return subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        return "nogit"


def run_id() -> str:
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return f"{timestamp}-{git_sha()}"


async def run_benchmark(
    examples: list[tuple[int, dict[str, Any]]],
    vocabulary: dict[str, int],
    client: httpx.AsyncClient,
) -> tuple[dict[str, Any], list[dict[str, Any]], list[dict[str, Any]]]:
    records = []
    failures = []
    for query_index, example in examples:
        try:
            record, query_failures = await benchmark_example(
                query_index, example, vocabulary, client
            )
        except Exception as error:
            message = f"{type(error).__name__}: {error}"
            failed_method = {
                "retrieved_urls": [],
                **{metric: None for metric in METRICS},
                "error": message,
            }
            record = {
                "query_index": query_index,
                "query": example.get("query"),
                "question_type": example.get("question_type"),
                "expected_evidence_urls": [],
                "jev": failed_method.copy(),
                "rag": failed_method.copy(),
                "error": message,
            }
            query_failures = [{
                "query_index": query_index,
                "method": "benchmark",
                "error": message,
            }]
        records.append(record)
        failures.extend(query_failures)
    return aggregate_records(records), records, failures


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n")


def write_jsonl(path: Path, records: list[dict[str, Any]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("".join(json.dumps(record, ensure_ascii=False) + "\n" for record in records))


def positive_int(value: str) -> int:
    parsed = int(value)
    if parsed < 1:
        raise argparse.ArgumentTypeError("must be positive")
    return parsed


def nonnegative_int(value: str) -> int:
    parsed = int(value)
    if parsed < 0:
        raise argparse.ArgumentTypeError("must be non-negative")
    return parsed


def parser() -> argparse.ArgumentParser:
    command = argparse.ArgumentParser(description=__doc__)
    subcommands = command.add_subparsers(dest="command", required=True)
    subcommands.add_parser("index", help="index the MultiHopRAG corpus")
    run = subcommands.add_parser("run", help="run both retrieval benchmarks")
    run.add_argument("--limit", type=positive_int, required=True)
    run.add_argument("--offset", type=nonnegative_int, default=0)
    run.add_argument("--question-type")
    run.add_argument("--seed", type=int)
    run.add_argument("--output", type=Path)
    run.add_argument("--records", type=Path)
    return command


async def async_main(args: argparse.Namespace) -> None:
    timeout = httpx.Timeout(120, connect=10)
    async with httpx.AsyncClient(timeout=timeout) as client:
        if args.command == "index":
            documents, _ = prepare_corpus()
            await index_documents(
                documents,
                client,
                int(os.getenv("EMBEDDING_BATCH_SIZE", "64")),
            )
            return

        _, stats = prepare_corpus()
        examples = select_examples(
            load_json(EXAMPLES_PATH),
            args.offset,
            args.limit,
            args.question_type,
            args.seed,
        )
        aggregate, records, failures = await run_benchmark(
            examples, stats["vocabulary"], client
        )
        output = {
            "run_id": run_id(),
            "dataset": "yixuantt/MultiHopRAG",
            "dataset_sha256": dataset_sha256(EXAMPLES_PATH),
            "collection": db.HYBRID_COLLECTION,
            "sample": {
                "offset": args.offset,
                "limit": args.limit,
                "question_type": args.question_type,
                "seed": args.seed,
            },
            "configuration": {
                "retrieval_limit": RETRIEVAL_LIMIT,
                "result_limit": RESULT_LIMIT,
                "bm25_k1": BM25_K1,
                "bm25_b": BM25_B,
                "embedding_token_limit": EMBEDDING_TOKEN_LIMIT,
                "embedding_model": os.getenv("OPENROUTER_EMBEDDING_MODEL"),
                "jev_model": os.getenv("OPENROUTER_JEV_MODEL", "~typesafe/jev-latest"),
                "question_model": os.getenv("OPENROUTER_QUESTION_MODEL"),
            },
            **aggregate,
            "failures": failures,
        }
        if args.output:
            write_json(args.output, output)
        else:
            print(json.dumps(output, indent=2, ensure_ascii=False))
        if args.records:
            write_jsonl(args.records, records)


def main(argv: list[str] | None = None) -> None:
    args = parser().parse_args(argv)
    asyncio.run(async_main(args))


if __name__ == "__main__":
    main()
