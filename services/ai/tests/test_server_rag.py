"""Tests for hub_ai._server RAG methods (RAGSearch, IndexDocument) and LLM provider."""

import os
import sys
import pytest
from unittest.mock import patch, MagicMock, AsyncMock

# Restore real grpc if test_main.py has already replaced it with a mock
# (pytest collects all modules before running, so test_main's module-level
# sys.modules["grpc"] = MagicMock() poisons subsequent imports).
_grpc_saved = sys.modules.pop("grpc", None)

# Pre-mock rag.knowledge_base so lazy imports inside servicer methods
# resolve to our mock instead of loading chromadb + langchain deps.
_rag_kb_mock = MagicMock()
sys.modules["rag.knowledge_base"] = _rag_kb_mock

from hub_ai._server import HubAIServicer  # noqa: E402


def _make_context():
    """Create a mock gRPC context with invocation_metadata."""
    ctx = MagicMock()
    ctx.invocation_metadata.return_value = [("x-trace-id", "test-trace")]
    return ctx


class TestRAGSearch:
    """Test RAGSearch RPC handler."""

    @pytest.mark.asyncio
    async def test_returns_answer_with_sources_when_snippets_found(self):
        servicer = HubAIServicer()
        ctx = _make_context()

        mock_kb_instance = MagicMock()
        mock_snippets = [
            {
                "content": "Chunks about embeddings.",
                "metadata": {"doc_id": "doc-1", "title": "Embeddings Guide", "chunk_index": 0},
                "distance": 0.15,
            },
            {
                "content": "More about vector search.",
                "metadata": {"doc_id": "doc-1", "title": "Embeddings Guide", "chunk_index": 1},
                "distance": 0.25,
            },
        ]
        mock_kb_instance.search.return_value = mock_snippets
        _rag_kb_mock.KnowledgeBase.return_value = mock_kb_instance
        servicer._llm.ask = AsyncMock(return_value="Embeddings are vector representations.")

        from nl2sql.v1 import nl2sql_pb2  # noqa: E402

        request = nl2sql_pb2.RAGSearchRequest(
            workspace_id="ws-1",
            question="What are embeddings?",
            top_k=3,
        )
        response = await servicer.RAGSearch(request, ctx)

        assert response.ok
        assert response.answer == "Embeddings are vector representations."
        assert len(response.sources) == 2
        assert response.sources[0].doc_id == "doc-1"
        assert response.sources[0].title == "Embeddings Guide"
        assert response.sources[1].chunk_index == 1

    @pytest.mark.asyncio
    async def test_returns_answer_with_empty_sources_when_no_snippets(self):
        servicer = HubAIServicer()
        ctx = _make_context()

        mock_kb_instance = MagicMock()
        mock_kb_instance.search.return_value = []
        _rag_kb_mock.KnowledgeBase.return_value = mock_kb_instance
        servicer._llm.ask = AsyncMock(return_value="I don't know about that.")

        from nl2sql.v1 import nl2sql_pb2  # noqa: E402

        request = nl2sql_pb2.RAGSearchRequest(
            workspace_id="ws-1",
            question="What is unknown?",
            top_k=3,
        )
        response = await servicer.RAGSearch(request, ctx)

        assert response.ok
        assert response.answer == "I don't know about that."
        assert len(response.sources) == 0

    @pytest.mark.asyncio
    async def test_returns_error_on_exception(self):
        servicer = HubAIServicer()
        ctx = _make_context()

        mock_kb_instance = MagicMock()
        mock_kb_instance.search.side_effect = Exception("ChromaDB connection refused")
        _rag_kb_mock.KnowledgeBase.return_value = mock_kb_instance

        from nl2sql.v1 import nl2sql_pb2  # noqa: E402

        request = nl2sql_pb2.RAGSearchRequest(
            workspace_id="ws-1",
            question="test",
            top_k=3,
        )
        response = await servicer.RAGSearch(request, ctx)

        assert not response.ok
        assert "ChromaDB connection refused" in response.error_message
        assert response.answer == ""


class TestIndexDocument:
    """Test IndexDocument RPC handler."""

    @pytest.mark.asyncio
    async def test_indexes_document_successfully(self):
        servicer = HubAIServicer()
        ctx = _make_context()

        mock_kb_instance = MagicMock()
        mock_kb_instance.add_document.return_value = 5
        _rag_kb_mock.KnowledgeBase.return_value = mock_kb_instance

        from nl2sql.v1 import nl2sql_pb2  # noqa: E402

        request = nl2sql_pb2.IndexDocumentRequest(
            workspace_id="ws-1",
            doc_id="doc-abc",
            title="Test Doc",
            text_content="Some long document content here...",
        )
        response = await servicer.IndexDocument(request, ctx)

        assert response.ok
        assert response.chunk_count == 5
        assert response.error_message == ""
        mock_kb_instance.add_document.assert_called_once_with(
            doc_id="doc-abc",
            title="Test Doc",
            text_content="Some long document content here...",
        )

    @pytest.mark.asyncio
    async def test_returns_error_on_failure(self):
        servicer = HubAIServicer()
        ctx = _make_context()

        mock_kb_instance = MagicMock()
        mock_kb_instance.add_document.side_effect = Exception("Indexing failed")
        _rag_kb_mock.KnowledgeBase.return_value = mock_kb_instance

        from nl2sql.v1 import nl2sql_pb2  # noqa: E402

        request = nl2sql_pb2.IndexDocumentRequest(
            workspace_id="ws-1",
            doc_id="doc-abc",
            title="Test Doc",
            text_content="content",
        )
        response = await servicer.IndexDocument(request, ctx)

        assert not response.ok
        assert response.chunk_count == 0
        assert "Indexing failed" in response.error_message


class TestOpenAIProviderAsk:
    """Test OpenAIProvider.ask method."""

    @pytest.mark.asyncio
    async def test_with_context(self):
        from llm_provider import OpenAIProvider

        mock_create = AsyncMock()
        mock_create.return_value.choices = [
            MagicMock(message=MagicMock(content="Answer based on context."))
        ]

        with patch.dict(os.environ, {"OPENAI_API_KEY": "test-key"}, clear=True), \
             patch("openai.AsyncOpenAI") as mock_client_cls:
            mock_client = MagicMock()
            mock_client.chat.completions.create = mock_create
            mock_client_cls.return_value = mock_client

            provider = OpenAIProvider()
            answer = await provider.ask(
                question="What is RAG?",
                context="RAG stands for Retrieval-Augmented Generation.",
            )

            assert answer == "Answer based on context."
            call_kwargs = mock_create.call_args[1]
            assert len(call_kwargs["messages"]) == 2
            assert call_kwargs["messages"][0]["role"] == "system"
            assert "RAG stands for" in call_kwargs["messages"][0]["content"]

    @pytest.mark.asyncio
    async def test_without_context(self):
        from llm_provider import OpenAIProvider

        mock_create = AsyncMock()
        mock_create.return_value.choices = [
            MagicMock(message=MagicMock(content="Answer from knowledge."))
        ]

        with patch.dict(os.environ, {"OPENAI_API_KEY": "test-key"}, clear=True), \
             patch("openai.AsyncOpenAI") as mock_client_cls:
            mock_client = MagicMock()
            mock_client.chat.completions.create = mock_create
            mock_client_cls.return_value = mock_client

            provider = OpenAIProvider()
            answer = await provider.ask(
                question="What is Python?",
                context="",
            )

            assert answer == "Answer from knowledge."
            call_kwargs = mock_create.call_args[1]
            system_content = call_kwargs["messages"][0]["content"]
            assert "如果不知道答案" in system_content
