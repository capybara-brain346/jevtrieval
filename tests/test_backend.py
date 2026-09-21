import json
import os
import tempfile
import unittest
from unittest.mock import AsyncMock, patch

from fastapi.testclient import TestClient

from backend import db, jev
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

    async def test_jev_probability_shape(self):
        result = jev._probabilities(
            {"answers": {"q_0": {"type": "noul", "noul": 0.75}}},
            ["Relevant?"],
        )
        self.assertEqual(result, [{"question": "Relevant?", "probability": 0.75, "error": None}])


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
        scored = {**document, "probabilities": [{"question": "Relevant?", "probability": 0.75, "error": None}]}
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
