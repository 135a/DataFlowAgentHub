import pytest
from unittest.mock import patch, MagicMock
import sys
import os

# 添加父目录到路径以允许导入
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


class TestKnowledgeBase:
    """对 rag.knowledge_base 中 KnowledgeBase 类的测试"""

    @patch("rag.knowledge_base.chromadb")
    @patch("rag.knowledge_base.OpenAIEmbeddings")
    def test_chunk_document(self, mock_embeddings_cls, mock_chromadb):
        """添加比 chunk_size 更长的文档应生成多个分块。"""
        from rag.knowledge_base import KnowledgeBase

        # 设置 chromadb 模拟
        mock_client = MagicMock()
        mock_collection = MagicMock()
        mock_collection.count.return_value = 0
        mock_client.get_or_create_collection.return_value = mock_collection
        mock_chromadb.HttpClient.return_value = mock_client

        # 设置嵌入模拟
        mock_embeddings = MagicMock()
        mock_embeddings_cls.return_value = mock_embeddings

        kb = KnowledgeBase("test-workspace")

        # 创建一个长文档（约 2500 字符，chunk_size=1000, overlap=200 时应产生约 3 个分块）
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
        """短文档可能产生 0 或 1 个分块，具体取决于分割器行为。"""
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

        # 短文本最多应产生 1 个分块
        assert chunk_count <= 1

    @patch("rag.knowledge_base.chromadb")
    @patch("rag.knowledge_base.OpenAIEmbeddings")
    def test_empty_document(self, mock_embeddings_cls, mock_chromadb):
        """空字符串应返回 0 个分块。"""
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
        """在空集合上搜索应返回空列表。"""
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
