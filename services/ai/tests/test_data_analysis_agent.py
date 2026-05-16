import pytest
import pandas as pd
import numpy as np
from unittest.mock import patch, MagicMock
import sys
import os

# Add parent directory to path to allow imports
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from agents.data_analysis_agent import data_analysis_node, truncate_data


class TestTruncateData:
    def test_no_truncation_when_under_limit(self):
        data = [{"a": i} for i in range(10)]
        result = truncate_data(data, max_rows=500)
        assert len(result) == 10

    def test_truncation_when_over_limit(self):
        data = [{"a": i} for i in range(1000)]
        result = truncate_data(data, max_rows=500)
        assert len(result) == 500

    def test_empty_list(self):
        result = truncate_data([], max_rows=500)
        assert result == []


class TestDataAnalysisNode:
    def test_empty_dataframe(self):
        """When nl2sql_result is empty, return appropriate message."""
        state = {
            "run_id": "test-run-1",
            "user_input": "test query",
            "nl2sql_result": [],
        }
        with patch("agents.data_analysis_agent.report_run_step"):
            result = data_analysis_node(state)
        assert "No data available" in result["analysis_summary"]

    def test_basic_stats(self):
        """With numeric data, stats should include describe output."""
        data = [
            {"name": "Alice", "score": 85, "age": 30},
            {"name": "Bob", "score": 92, "age": 25},
            {"name": "Charlie", "score": 78, "age": 35},
            {"name": "Diana", "score": 88, "age": 28},
            {"name": "Eve", "score": 95, "age": 32},
        ]
        state = {
            "run_id": "test-run-2",
            "user_input": "analyze scores",
            "nl2sql_result": data,
        }
        with patch("agents.data_analysis_agent.report_run_step"):
            result = data_analysis_node(state)

        summary = result["analysis_summary"]
        # Should contain numeric analysis (raw stats when no OpenAI key)
        assert "Numeric Columns Summary" in summary or "std" in summary.lower() or "mean" in summary.lower() or "LLM" in summary

    def test_no_numeric_columns(self):
        """Non-numeric data should still produce a message."""
        data = [
            {"name": "Alice", "city": "NYC"},
            {"name": "Bob", "city": "LA"},
        ]
        state = {
            "run_id": "test-run-3",
            "user_input": "list names",
            "nl2sql_result": data,
        }
        with patch("agents.data_analysis_agent.report_run_step"):
            result = data_analysis_node(state)
        assert "analysis_summary" in result
