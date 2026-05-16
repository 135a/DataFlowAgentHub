import pytest
from unittest.mock import patch, MagicMock
import sys
import os

# Add parent directory to path to allow imports
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


class TestKnowledgeBase:
    """Tests for KnowledgeBase class from rag.knowledge_base"""

    @patch("rag.knowledge_base.chromadb")
    @patch("rag.knowledge_base.OpenAIEmbeddings")
    def test_chunk_document(self, mock_embeddings_cls, mock_chromadb):
        """Adding a document longer than chunk_size should produce multiple chunks."""
        from rag.knowledge_base import KnowledgeBase

        # Setup chromadb mock
        mock_client = MagicMock()
        mock_collection = MagicMock()
        mock_collection.count.return_value = 0
        mock_client.get_or_create_collection.return_value = mock_collection
        mock_chromadb.HttpClient.return_value = mock_client

        # Setup embeddings mock
        mock_embeddings = MagicMock()
        mock_embeddings_cls.return_value = mock_embeddings

        kb = KnowledgeBase("test-workspace")

        # Create a long document (~2500 chars, should yield ~3 chunks with chunk_size=1000, overlap=200)
        long_text = "This is a test document. " * 200
        chunk_count = kb.add_document("doc-1", "Test Doc", long_text)

        assert chunk_count > 1
        assert mock_collection.add.called
        call_args = mock_collection.add.call_args[1]
        assert len(call_args["ids"]) == chunk_count
        assert len(call_args["documents"]) == chunk_count
        assert len(call_args["metadatas"]) == chunk_count

    @patch("rag.knowledge_base.chromadb")
    @patch("rag.knowledge_base.OpenAIEmbeddings")
    def test_chunk_short_document(self, mock_embeddings_cls, mock_chromadb):
        """A short document may produce 0 or 1 chunk depending on splitter behavior."""
        from rag.knowledge_base import KnowledgeBase

        mock_client = MagicMock()
        mock_collection = MagicMock()
        mock_collection.count.return_value = 0
        mock_client.get_or_create_collection.return_value = mock_collection
        mock_chromadb.HttpClient.return_value = mock_client

        mock_embeddings = MagicMock()
        mock_embeddings_cls.return_value = mock_embeddings

        kb = KnowledgeBase("test-workspace-short")

        short_text = "Hello world"
        chunk_count = kb.add_document("doc-2", "Short Doc", short_text)

        # Short text should produce at most 1 chunk
        assert chunk_count <= 1

    @patch("rag.knowledge_base.chromadb")
    @patch("rag.knowledge_base.OpenAIEmbeddings")
    def test_empty_document(self, mock_embeddings_cls, mock_chromadb):
        """Empty string should return 0 chunks."""
        from rag.knowledge_base import KnowledgeBase

        mock_client = MagicMock()
        mock_collection = MagicMock()
        mock_client.get_or_create_collection.return_value = mock_collection
        mock_chromadb.HttpClient.return_value = mock_client

        mock_embeddings = MagicMock()
        mock_embeddings_cls.return_value = mock_embeddings

        kb = KnowledgeBase("test-workspace-empty")
        chunk_count = kb.add_document("doc-3", "Empty Doc", "")

        assert chunk_count == 0

    @patch("rag.knowledge_base.chromadb")
    @patch("rag.knowledge_base.OpenAIEmbeddings")
    def test_search_empty_collection(self, mock_embeddings_cls, mock_chromadb):
        """Search on an empty collection should return empty list."""
        from rag.knowledge_base import KnowledgeBase

        mock_client = MagicMock()
        mock_collection = MagicMock()
        mock_collection.count.return_value = 0
        mock_client.get_or_create_collection.return_value = mock_collection
        mock_chromadb.HttpClient.return_value = mock_client

        mock_embeddings = MagicMock()
        mock_embeddings_cls.return_value = mock_embeddings

        kb = KnowledgeBase("test-workspace-search")
        results = kb.search("test query")

        assert results == []
        mock_collection.query.assert_not_called()
