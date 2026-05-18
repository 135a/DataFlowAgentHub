"""Tests for orchestrator/knowledge_consumer.py"""

import asyncio
import json
import sys
import pytest
from unittest.mock import patch, MagicMock, AsyncMock

# Mock NATS before any consumer imports
sys.modules["nats"] = MagicMock()
sys.modules["nats.aio"] = MagicMock()
sys.modules["nats.aio.client"] = MagicMock()
sys.modules["nats.aio.client.Client"] = MagicMock


class TestProcessKnowledgeMessage:
    """Test process_knowledge_message function."""

    def _import(self):
        """Import knowledge_consumer module with proper mocking."""
        for mod_name in [
            "orchestrator.knowledge_consumer",
            "hub_ai._client",
            "rag.knowledge_base",
        ]:
            if mod_name in sys.modules:
                del sys.modules[mod_name]

        # Mock KnowledgeBase and get_client at module level
        with patch("orchestrator.knowledge_consumer.KnowledgeBase"), \
             patch("orchestrator.knowledge_consumer.get_client"):
            from orchestrator.knowledge_consumer import process_knowledge_message
            return process_knowledge_message

    @pytest.mark.asyncio
    async def test_indexes_document_successfully(self):
        """Valid document should be indexed and acked."""
        process_fn = self._import()
        mock_client = AsyncMock()
        mock_client.knowledge_doc_callback = AsyncMock()
        mock_kb = MagicMock()
        mock_kb.add_document.return_value = "chroma-doc-123"

        msg = MagicMock()
        msg.data = json.dumps({
            "workspace_id": "ws-001",
            "payload": {
                "doc_id": "doc-001",
                "title": "Test Document",
                "content": "This is the document content for testing.",
            },
        }).encode()
        msg.ack = AsyncMock()
        msg.nak = AsyncMock()

        with patch("orchestrator.knowledge_consumer.get_client",
                   return_value=mock_client), \
             patch("orchestrator.knowledge_consumer.KnowledgeBase",
                   return_value=mock_kb):
            await process_fn(msg)

            # Verify KnowledgeBase was created with correct workspace
            mock_kb.add_document.assert_called_once_with(
                doc_id="doc-001",
                title="Test Document",
                text_content="This is the document content for testing.",
            )

            # Verify gRPC callback was called with success
            mock_client.knowledge_doc_callback.assert_called_once_with(
                doc_id="doc-001",
                status="completed",
                chroma_doc_id="chroma-doc-123",
                chunk_count=0,
            )

            # Verify message was acknowledged
            msg.ack.assert_called_once()

    @pytest.mark.asyncio
    async def test_handles_missing_content(self):
        """Missing content should skip indexing and ack."""
        process_fn = self._import()
        msg = MagicMock()
        msg.data = json.dumps({
            "workspace_id": "ws-001",
            "payload": {
                "doc_id": "doc-002",
                "title": "Empty Doc",
                "content": "",
            },
        }).encode()
        msg.ack = AsyncMock()
        msg.nak = AsyncMock()

        await process_fn(msg)
        msg.ack.assert_called_once()
        msg.nak.assert_not_called()

    @pytest.mark.asyncio
    async def test_handles_missing_workspace_id(self):
        """Missing workspace_id should skip indexing and ack."""
        process_fn = self._import()
        msg = MagicMock()
        msg.data = json.dumps({
            "payload": {
                "doc_id": "doc-003",
                "title": "Test",
                "content": "Some content.",
            },
        }).encode()
        msg.ack = AsyncMock()
        msg.nak = AsyncMock()

        await process_fn(msg)
        msg.ack.assert_called_once()
        msg.nak.assert_not_called()

    @pytest.mark.asyncio
    async def test_handles_chromadb_error(self):
        """ChromaDB error should trigger failure callback and nak."""
        process_fn = self._import()
        mock_client = AsyncMock()
        mock_client.knowledge_doc_callback = AsyncMock()
        mock_kb = MagicMock()
        mock_kb.add_document.side_effect = Exception("ChromaDB connection failed")

        msg = MagicMock()
        msg.data = json.dumps({
            "workspace_id": "ws-001",
            "payload": {
                "doc_id": "doc-004",
                "title": "Failing Doc",
                "content": "Some content that will fail.",
            },
        }).encode()
        msg.ack = AsyncMock()
        msg.nak = AsyncMock()

        with patch("orchestrator.knowledge_consumer.get_client",
                   return_value=mock_client), \
             patch("orchestrator.knowledge_consumer.KnowledgeBase",
                   return_value=mock_kb):
            await process_fn(msg)

            # Verify failure callback was sent
            mock_client.knowledge_doc_callback.assert_called_once_with(
                doc_id="doc-004",
                status="failed",
                chunk_count=0,
            )

            # Verify message was negatively acknowledged
            msg.nak.assert_called_once()

    @pytest.mark.asyncio
    async def test_handles_invalid_json(self):
        """Invalid JSON payload should not crash."""
        process_fn = self._import()
        msg = MagicMock()
        msg.data = b"not valid json at all"
        msg.ack = AsyncMock()
        msg.nak = AsyncMock()

        await process_fn(msg)
        # With empty doc_id, the except block checks if doc_id is truthy
        # Since it's empty, it won't call failure callback, just nak
        msg.nak.assert_called_once()

    @pytest.mark.asyncio
    async def test_failure_callback_error_does_not_raise(self):
        """If the failure callback itself fails, it should not propagate."""
        process_fn = self._import()
        mock_client = AsyncMock()
        mock_client.knowledge_doc_callback = AsyncMock(
            side_effect=Exception("callback also failed")
        )
        mock_kb = MagicMock()
        mock_kb.add_document.side_effect = Exception("ChromaDB error")

        msg = MagicMock()
        msg.data = json.dumps({
            "workspace_id": "ws-001",
            "payload": {
                "doc_id": "doc-005",
                "title": "Double Fail",
                "content": "Content.",
            },
        }).encode()
        msg.ack = AsyncMock()
        msg.nak = AsyncMock()

        with patch("orchestrator.knowledge_consumer.get_client",
                   return_value=mock_client), \
             patch("orchestrator.knowledge_consumer.KnowledgeBase",
                   return_value=mock_kb):
            # Should not raise despite both failures
            await process_fn(msg)
            msg.nak.assert_called_once()

    @pytest.mark.asyncio
    async def test_nak_failure_does_not_raise(self):
        """If msg.nak() itself fails, it should not propagate."""
        process_fn = self._import()
        mock_kb = MagicMock()
        mock_kb.add_document.side_effect = Exception("ChromaDB error")

        msg = MagicMock()
        msg.data = json.dumps({
            "workspace_id": "ws-001",
            "payload": {
                "doc_id": "doc-006",
                "title": "Nak Fail",
                "content": "Content.",
            },
        }).encode()
        msg.ack = AsyncMock()
        msg.nak = AsyncMock(side_effect=Exception("nak failed"))

        with patch("orchestrator.knowledge_consumer.get_client",
                   return_value=AsyncMock()), \
             patch("orchestrator.knowledge_consumer.KnowledgeBase",
                   return_value=mock_kb):
            # Should not raise despite nak failure
            await process_fn(msg)


class TestRunKnowledgeConsumer:
    """Test run_knowledge_consumer function."""

    def _import_consumer(self):
        """Import the run_knowledge_consumer function."""
        for mod_name in [
            "orchestrator.knowledge_consumer",
            "hub_ai._client",
            "rag.knowledge_base",
        ]:
            if mod_name in sys.modules:
                del sys.modules[mod_name]

        with patch("orchestrator.knowledge_consumer.KnowledgeBase"), \
             patch("orchestrator.knowledge_consumer.get_client"):
            from orchestrator.knowledge_consumer import run_knowledge_consumer
            return run_knowledge_consumer

    @pytest.mark.asyncio
    async def test_connects_and_subscribes(self):
        """Should connect to NATS and subscribe to knowledge_index."""
        run_consumer = self._import_consumer()
        mock_nc = MagicMock()
        mock_nc.is_connected = True
        mock_nc.connect = AsyncMock()
        mock_nc.subscribe = AsyncMock()
        mock_nc.drain = AsyncMock()

        with patch("orchestrator.knowledge_consumer.NATS", return_value=mock_nc), \
             patch("asyncio.Future", side_effect=Exception("stop loop")):
            with pytest.raises(Exception, match="stop loop"):
                await run_consumer("nats://test:4222")

            mock_nc.connect.assert_called_once_with("nats://test:4222")
            # Verify subscribe was called with the right topic and a callable callback
            mock_nc.subscribe.assert_called_once()
            args, kwargs = mock_nc.subscribe.call_args
            assert args[0] == "hub.tasks.knowledge_index"
            assert callable(kwargs["cb"])

    @pytest.mark.asyncio
    async def test_reconnects_on_failure(self):
        """Should retry connection on failure with exponential backoff."""
        run_consumer = self._import_consumer()
        mock_nc = MagicMock()
        mock_nc.is_connected = True

        # Track connection attempts: first fails, second succeeds
        connect_results = [Exception("Connection refused"), None]
        mock_nc.connect = AsyncMock(side_effect=connect_results)
        mock_nc.subscribe = AsyncMock()
        mock_nc.drain = AsyncMock()

        with patch("orchestrator.knowledge_consumer.NATS", return_value=mock_nc), \
             patch("asyncio.Future", side_effect=Exception("stop loop")):
            with pytest.raises(Exception, match="stop loop"):
                await run_consumer("nats://test:4222")

            # connect should have been called twice (1 fail + 1 success)
            assert mock_nc.connect.call_count == 2

    @pytest.mark.asyncio
    async def test_gives_up_after_max_retries(self):
        """Should raise after exhausting all retry attempts."""
        run_consumer = self._import_consumer()
        mock_nc = MagicMock()
        mock_nc.is_connected = True
        mock_nc.connect = AsyncMock(side_effect=Exception("Connection refused"))
        mock_nc.drain = AsyncMock()

        with patch("orchestrator.knowledge_consumer.NATS", return_value=mock_nc):
            with pytest.raises(Exception):
                await run_consumer("nats://test:4222")

            # Should have attempted connection 5 times (max_retries)
            assert mock_nc.connect.call_count == 5

    @pytest.mark.asyncio
    async def test_handles_cancelled_error(self):
        """CancelledError should gracefully drain and exit."""
        run_consumer = self._import_consumer()
        mock_nc = MagicMock()
        mock_nc.is_connected = True
        mock_nc.connect = AsyncMock()
        mock_nc.drain = AsyncMock()

        with patch("orchestrator.knowledge_consumer.NATS", return_value=mock_nc), \
             patch("asyncio.Future", side_effect=asyncio.CancelledError()):
            await run_consumer("nats://test:4222")
            mock_nc.drain.assert_called_once()
