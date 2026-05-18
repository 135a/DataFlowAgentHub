"""Tests for orchestrator/consumer.py"""

import json
import sys
import pytest
from unittest.mock import patch, MagicMock, AsyncMock

# Mock NATS before any consumer imports
sys.modules["nats"] = MagicMock()
sys.modules["nats.aio"] = MagicMock()
sys.modules["nats.aio.client"] = MagicMock()
sys.modules["nats.aio.client.Client"] = MagicMock

# Mock the missing sqlite submodule
sys.modules["langgraph.checkpoint.sqlite"] = MagicMock()


class TestProcessMessage:
    """Test consumer process_message function."""

    def _import_consumer(self):
        """Import consumer module with proper mocking."""
        if "orchestrator.consumer" in sys.modules:
            del sys.modules["orchestrator.consumer"]
        if "orchestrator.graph" in sys.modules:
            del sys.modules["orchestrator.graph"]

        with patch("orchestrator.consumer.get_client"):
            from orchestrator.consumer import process_message
            return process_message

    @pytest.mark.asyncio
    async def test_processes_valid_message(self):
        """Valid message should invoke graph and call gRPC callback."""
        process_message = self._import_consumer()
        mock_client = AsyncMock()
        mock_client.task_callback = AsyncMock()

        msg = MagicMock()
        msg.data = json.dumps({
            "id": "task-001",
            "session_id": "session-1",
            "run_id": "run-001",
            "payload": {"user_message": "analyze sales", "schema_json": "{}"},
        }).encode()
        msg.ack = AsyncMock()
        msg.nak = AsyncMock()

        with patch("orchestrator.consumer.get_client", return_value=mock_client), \
             patch("orchestrator.consumer.workflow_graph.ainvoke") as mock_invoke:
            mock_invoke = AsyncMock(return_value={
                "final_report": "Report content",
                "analysis_summary": "Sales up 20%",
            })
            with patch("orchestrator.consumer.workflow_graph.ainvoke", mock_invoke):
                await process_message(msg)

                assert mock_invoke.called
                mock_client.task_callback.assert_called_once()

    @pytest.mark.asyncio
    async def test_handles_invalid_payload(self):
        """Invalid JSON payload should not crash."""
        process_message = self._import_consumer()
        msg = MagicMock()
        msg.data = b"not valid json"
        msg.ack = AsyncMock()
        msg.nak = AsyncMock()

        await process_message(msg)
        assert msg.nak.called

    @pytest.mark.asyncio
    async def test_handles_graph_timeout(self):
        """When graph.ainvoke fails, should report failure."""
        process_message = self._import_consumer()
        mock_client = AsyncMock()
        mock_client.task_callback = AsyncMock()

        msg = MagicMock()
        msg.data = json.dumps({
            "id": "task-002",
            "session_id": "session-1",
            "run_id": "run-002",
            "payload": {"user_message": "test", "schema_json": "{}"},
        }).encode()
        msg.ack = AsyncMock()
        msg.nak = AsyncMock()

        with patch("orchestrator.consumer.get_client", return_value=mock_client), \
             patch("orchestrator.consumer.workflow_graph.ainvoke",
                   side_effect=Exception("Graph execution failed")):
            await process_message(msg)

            mock_client.task_callback.assert_called_with(
                task_id="task-002",
                status="failed",
                error_message="Graph execution failed",
            )
