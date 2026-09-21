import json
import os
import tempfile
import unittest
from unittest.mock import AsyncMock, patch

from fastapi.testclient import TestClient

from backend import db, jev, llm
from backend.app import app


class Response:
    def __init__(self, data, status_code=200):
        self._data = data
        self.status_code = status_code
        self.is_success = status_code < 400

    def json(self):
        return self._data

    def raise_for_status(self):
        if not self.is_success:
            raise RuntimeError(self.status_code)


class BackendTest(unittest.IsolatedAsyncioTestCase):
    async def test_qdrant_results_are_ranked_and_normalized(self):
        client = AsyncMock()
        client.post.return_value = Response({"result": {"points": [
            {"id": 7, "score": 0.9, "payload": {"title": "Seven", "body": "Text"}},
        ]}})

        documents = await db.search([0.1], 1, client)

        self.assertEqual(documents, [{
            "doc_id": "7", "title": "Seven", "text": "Text", "rank": 1, "vector_score": 0.9,
        }])

    async def test_hybrid_collection_is_created_with_named_vectors(self):
        client = AsyncMock()
        client.get.return_value = Response({}, status_code=404)
        client.put.return_value = Response({"result": True})

        await db.ensure_hybrid_collection(3, client)

        self.assertEqual(
            client.put.await_args.kwargs["json"],
            {
                "vectors": {"dense": {"size": 3, "distance": "Cosine"}},
                "sparse_vectors": {"bm25": {}},
            },
        )

    async def test_existing_hybrid_schema_is_not_mutated(self):
        client = AsyncMock()
        client.get.return_value = Response({
            "result": {
                "config": {
                    "params": {
                        "vectors": {"dense": {"size": 3, "distance": "Cosine"}},
                        "sparse_vectors": {"bm25": {}},
                    }
                }
            }
        })

        await db.ensure_hybrid_collection(3, client)

        client.put.assert_not_awaited()

    async def test_incompatible_hybrid_schema_raises(self):
        client = AsyncMock()
        client.get.return_value = Response({
            "result": {
                "config": {
                    "params": {
                        "vectors": {"dense": {"size": 2, "distance": "Cosine"}},
                        "sparse_vectors": {"bm25": {}},
                    }
                }
            }
        })

        with self.assertRaises(ValueError):
            await db.ensure_hybrid_collection(3, client)

        client.put.assert_not_awaited()

    async def test_hybrid_upsert_keeps_uuid_and_url_separate(self):
        client = AsyncMock()
        client.put.return_value = Response({"result": True})

        await db.upsert_hybrid([{
            "point_id": "550e8400-e29b-41d4-a716-446655440000",
            "doc_id": "https://example.test/article",
            "vector": [0.1, 0.2],
            "sparse_vector": {"indices": [4], "values": [1.5]},
            "title": "Title",
            "text": "Text",
        }], client)

        point = client.put.await_args.kwargs["json"]["points"][0]
        self.assertEqual(point["id"], "550e8400-e29b-41d4-a716-446655440000")
        self.assertEqual(point["payload"]["doc_id"], "https://example.test/article")
        self.assertEqual(point["vector"]["bm25"], {"indices": [4], "values": [1.5]})

    async def test_hybrid_search_uses_rrf_prefetches_and_final_order(self):
        client = AsyncMock()
        client.post.return_value = Response({"result": {"points": [
            {"id": "b", "score": 0.8, "payload": {"doc_id": "url-b"}},
            {"id": "a", "score": 0.7, "payload": {"doc_id": "url-a"}},
        ]}})

        documents = await db.hybrid_search(
            [0.1], {"indices": [2], "values": [1.0]}, 25, 5, client
        )

        query = client.post.await_args.kwargs["json"]
        self.assertEqual([prefetch["limit"] for prefetch in query["prefetch"]], [25, 25])
        self.assertEqual([prefetch["using"] for prefetch in query["prefetch"]], ["dense", "bm25"])
        self.assertEqual(query["query"], {"fusion": "rrf"})
        self.assertEqual(query["limit"], 5)
        self.assertEqual([document["doc_id"] for document in documents], ["url-b", "url-a"])
        self.assertEqual([document["rank"] for document in documents], [1, 2])
        self.assertEqual([document["vector_score"] for document in documents], [0.8, 0.7])

    async def test_existing_hybrid_point_lookup_returns_ids(self):
        client = AsyncMock()
        client.post.return_value = Response({"result": [{"id": "a"}]})

        result = await db.existing_point_ids(["a", "b"], client)

        self.assertEqual(result, {"a"})
        self.assertEqual(client.post.await_args.kwargs["json"]["ids"], ["a", "b"])

    async def test_jev_probability_shape(self):
        result = jev._probabilities(
            {"answers": {"q_0": {"type": "noul", "noul": 0.75}}},
            ["Relevant?"],
        )
        self.assertEqual(result, [{"question": "Relevant?", "probability": 0.75, "error": None}])

    async def test_jev_scores_direct_query_separately(self):
        probabilities = [
            {"question": "direct", "probability": 0.9, "error": None},
            {"question": "Helper?", "probability": 0.6, "error": None},
        ]
        with patch("backend.jev._score", AsyncMock(return_value=probabilities)) as score:
            result = await jev.score_documents(
                [{"doc_id": "7", "title": "Seven", "text": "Text"}],
                "User query",
                ["Helper?"],
                AsyncMock(),
            )

        self.assertEqual(result[0]["query_probability"], 0.9)
        self.assertEqual(result[0]["probabilities"], probabilities[1:])
        self.assertIn("User query", score.await_args.args[1][0])

    def test_selection_prioritizes_query_relevance_then_missing_coverage(self):
        def document(doc_id, relevance, probabilities, vector_score):
            return {
                "doc_id": doc_id,
                "query_probability": relevance,
                "vector_score": vector_score,
                "probabilities": [
                    {"question": str(index), "probability": probability, "error": None}
                    for index, probability in enumerate(probabilities)
                ],
            }

        selected = jev.select_documents(
            [
                document("A", 1.0, [1.0, 0.0], 0.9),
                document("B", 0.9, [1.0, 0.0], 0.8),
                document("C", 0.7, [0.0, 1.0], 0.7),
            ],
            limit=2,
        )

        self.assertEqual([document["doc_id"] for document in selected], ["A", "C"])

    def test_selection_returns_five_documents_below_relevance_threshold(self):
        documents = [
            {
                "doc_id": str(index),
                "query_probability": 0.06,
                "vector_score": 1 - index / 10,
                "probabilities": [
                    {"question": "helper", "probability": 0.06, "error": None}
                ],
            }
            for index in range(6)
        ]

        selected = jev.select_documents(documents)

        self.assertEqual([document["doc_id"] for document in selected], ["0", "1", "2", "3", "4"])

    async def test_answer_llm_receives_document_content_without_metadata(self):
        client = AsyncMock()
        client.post.return_value = Response({
            "choices": [{"message": {"content": "Answer"}}]
        })
        document = {
            "doc_id": "7",
            "title": "Seven",
            "text": "Only this content",
            "query_probability": 0.9,
            "probabilities": [],
        }

        with patch.dict(os.environ, {
            "OPENROUTER_API_KEY": "test",
            "OPENROUTER_ANSWER_MODEL": "test-model",
        }):
            await llm.generate_answer("Query", [document], client)

        payload = json.loads(
            client.post.await_args.kwargs["json"]["messages"][1]["content"]
        )
        self.assertEqual(payload, {
            "query": "Query",
            "documents": ["Only this content"],
        })


class ApiTest(unittest.TestCase):
    def test_index_embeds_and_upserts_local_documents(self):
        with tempfile.NamedTemporaryFile("w", delete=False) as source:
            source.write(json.dumps({"doc_id": "7", "title": "Seven", "text": "Text"}) + "\n")
        self.addCleanup(os.unlink, source.name)

        with (
            patch.dict(os.environ, {"SCIFACT_PATH": source.name}),
            patch("backend.app.db.count", AsyncMock(return_value=0)),
            patch("backend.app.embeddings.embed_many", AsyncMock(return_value=[[0.1, 0.2]])),
            patch("backend.app.db.ensure_collection", AsyncMock()) as ensure,
            patch("backend.app.db.upsert", AsyncMock()) as upsert,
            TestClient(app) as client,
        ):
            response = client.post("/v1/index")

        self.assertEqual(response.json(), {"indexed": 1, "skipped": False})
        ensure.assert_awaited_once()
        upsert.assert_awaited_once()

    def test_search_returns_one_complete_response(self):
        document = {"doc_id": "7", "title": "Seven", "text": "Text", "rank": 1, "vector_score": 0.9}
        scored = {
            **document,
            "query_probability": 0.8,
            "query_error": None,
            "probabilities": [
                {"question": "Relevant?", "probability": 0.75, "error": None},
                {"question": "Useful?", "probability": 0.5, "error": None},
            ],
        }
        with (
            patch("backend.app.llm.generate_questions", AsyncMock(return_value=["Relevant?", "Useful?"])),
            patch("backend.app.embeddings.embed", AsyncMock(return_value=[0.1])),
            patch("backend.app.db.search", AsyncMock(return_value=[document])),
            patch("backend.app.jev.score_documents", AsyncMock(return_value=[scored])),
            patch("backend.app.llm.generate_answer", AsyncMock(return_value="Answer [7]")),
            TestClient(app) as client,
        ):
            response = client.post("/v1/search", json={"query": " Test query "})

        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["answer"], "Answer [7]")
        self.assertEqual(response.json()["documents"], [scored])


if __name__ == "__main__":
    unittest.main()
