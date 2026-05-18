"""Tests for report_generation_agent.py"""

import pytest
from unittest.mock import patch, MagicMock, mock_open
import sys
import os

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


class TestReportGenerationNode:
    """Test report_generation_node function."""

    def _import_node(self):
        """Import with mocked dependencies."""
        with patch("agents.report_generation_agent.report_run_step"), \
             patch("agents.report_generation_agent.os.makedirs"), \
             patch("builtins.open", mock_open()):
            if "agents.report_generation_agent" in sys.modules:
                del sys.modules["agents.report_generation_agent"]
            from agents.report_generation_agent import report_generation_node
            return report_generation_node

    def test_generates_markdown_with_analysis_summary(self):
        """Markdown output should contain analysis summary."""
        node = self._import_node()
        state = {
            "run_id": "test-001",
            "user_input": "analyze sales",
            "analysis_summary": "Sales increased by 20% QoQ",
            "nl2sql_result": [{"product": "A", "revenue": 100}],
        }
        result = node(state)
        report = result["final_report"]
        assert "Sales increased by 20% QoQ" in report
        assert "analyze sales" in report

    def test_generates_markdown_with_chart_references(self):
        """When chart_paths exist, markdown should contain image references."""
        node = self._import_node()
        state = {
            "run_id": "test-002",
            "user_input": "show chart",
            "analysis_summary": "Summary",
            "nl2sql_result": [{"x": 1}],
            "chart_paths": ["/tmp/reports/test_chart_0.png"],
        }
        result = node(state)
        report = result["final_report"]
        assert "数据可视化" in report
        assert "test_chart_0.png" in report

    def test_handles_empty_data(self):
        """Empty data should still produce markdown noting no data."""
        node = self._import_node()
        state = {
            "run_id": "test-003",
            "user_input": "query",
            "analysis_summary": "Summary",
            "nl2sql_result": [],
        }
        result = node(state)
        report = result["final_report"]
        assert "No data returned" in report or "no data" in report.lower()

    def test_generates_excel_when_data_exists(self):
        """Excel file should be generated when result data exists."""
        # Use a real DataFrame to avoid mock issues with .to_markdown()
        import pandas as pd
        real_data = [{"col": "val"}]
        node = self._import_node()
        state = {
            "run_id": "test-004",
            "user_input": "export",
            "analysis_summary": "Summary",
            "nl2sql_result": real_data,
        }
        with patch("agents.report_generation_agent.pd.DataFrame.to_excel") as mock_to_excel:
            result = node(state)
            assert mock_to_excel.called
