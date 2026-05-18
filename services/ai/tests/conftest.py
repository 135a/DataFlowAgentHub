"""pytest fixtures for AI worker tests."""

import sys
import os

# Add both project root and gen/ to path so protobuf imports work
_test_dir = os.path.dirname(os.path.abspath(__file__))
_ai_dir = os.path.dirname(_test_dir)
sys.path.insert(0, _ai_dir)
sys.path.insert(0, os.path.join(_ai_dir, "gen"))

import pytest
import pandas as pd
import numpy as np
from unittest.mock import MagicMock, AsyncMock


def pytest_configure(config):
    """Register custom markers."""
    config.addinivalue_line("markers", "asyncio: mark test as async")


@pytest.fixture
def numeric_df():
    """DataFrame with numeric and categorical columns."""
    return pd.DataFrame({
        "name": ["Alice", "Bob", "Charlie", "Diana", "Eve"],
        "score": [85, 92, 78, 88, 95],
        "age": [30, 25, 35, 28, 32],
    })


@pytest.fixture
def categorical_df():
    """DataFrame with only non-numeric columns."""
    return pd.DataFrame({
        "name": ["Alice", "Bob", "Charlie"],
        "city": ["NYC", "LA", "Chicago"],
    })


@pytest.fixture
def mixed_df():
    """DataFrame with mixed types including date-like strings."""
    return pd.DataFrame({
        "month": ["2024-01", "2024-02", "2024-03", "2024-04", "2024-05"],
        "revenue": [10000, 12000, 11000, 13500, 14000],
        "cost": [7000, 8000, 7500, 9000, 9500],
    })


@pytest.fixture
def large_df():
    """DataFrame with more than 50 rows for truncation testing."""
    return pd.DataFrame({
        "id": range(100),
        "value": [f"row-{i}" for i in range(100)],
    })


@pytest.fixture
def empty_df():
    """Empty DataFrame."""
    return pd.DataFrame()


@pytest.fixture
def mock_grpc_client():
    """Mock HubInternalClient with async callback methods."""
    client = MagicMock()
    client.task_callback = MagicMock()
    client.run_step_callback = MagicMock()
    client.internal_nl2sql = MagicMock(return_value={
        "ok": True,
        "rows": [{"id": 1, "name": "test"}],
        "sql": "SELECT * FROM test",
    })
    client.knowledge_doc_callback = MagicMock()
    return client


@pytest.fixture
def mock_matplotlib(monkeypatch):
    """Mock matplotlib to prevent actual rendering in tests."""
    mock_plt = MagicMock()
    mock_fig = MagicMock()
    mock_ax = MagicMock()
    mock_plt.subplots.return_value = (mock_fig, mock_ax)
    monkeypatch.setattr("matplotlib.pyplot", mock_plt)
    return mock_plt


@pytest.fixture
def mock_nats_msg():
    """Create a mock NATS message with various payloads."""

    def _create(payload: dict) -> MagicMock:
        import json
        msg = MagicMock()
        msg.data = json.dumps(payload).encode()
        msg.ack = AsyncMock()
        msg.nak = AsyncMock()
        return msg

    return _create


@pytest.fixture
def sample_agent_state():
    """Standard agent state dict for graph testing."""
    return {
        "run_id": "test-run-001",
        "user_input": "analyze sales data",
        "schema_context": '{"tables": [{"name": "sales"}]}',
        "nl2sql_result": [{"product": "A", "revenue": 100}],
        "nl2sql_sql": "SELECT product, revenue FROM sales",
        "workflow": "agent_pipeline",
    }
