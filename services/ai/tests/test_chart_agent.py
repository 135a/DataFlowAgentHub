"""Tests for chart_agent.py

Uses importlib.reload() instead of fragile del sys.modules[...]
patching to support clean re-imports with mocked dependencies.
"""

import sys
import os
import importlib
import pytest
import pandas as pd
import numpy as np
from unittest.mock import patch, MagicMock

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# Pre-mock matplotlib modules to prevent import-time side effects
# from _configure_chinese_fonts() at module load time.
sys.modules["matplotlib"] = MagicMock()
sys.modules["matplotlib.pyplot"] = MagicMock()
sys.modules["matplotlib.font_manager"] = MagicMock()


def _reload_chart_agent():
    """Ensure chart_agent module is loaded and reload it.

    Uses importlib.reload() to force re-execution of the module
    code so that mocked dependencies (configured at the call site)
    take effect. This replaces the fragile del sys.modules[] pattern.
    """
    try:
        import agents.chart_agent
    except ImportError:
        # First import may fail if matplotlib isn't on the system;
        # we catch it gracefully so the reload below still works
        # after sys.modules has been seeded with mocks.
        pass
    return importlib.reload(sys.modules.get("agents.chart_agent"))


class TestSelectChartType:
    """Test _select_chart_type logic."""

    def _import_select(self):
        _reload_chart_agent()
        from agents.chart_agent import _select_chart_type
        return _select_chart_type

    def test_selects_bar_for_categorical_data(self):
        """Categorical x + numeric y with >6 rows should select bar."""
        _select = self._import_select()
        df = pd.DataFrame({
            "product": [f"P{i}" for i in range(10)],
            "sales": range(100, 110),
        })
        assert _select(df) == "bar"

    def test_selects_line_for_date_like_data(self):
        """Date-like column should select line chart."""
        _select = self._import_select()
        df = pd.DataFrame({
            "month": [f"2024-{m:02d}" for m in range(1, 11)],
            "revenue": range(100, 110),
        })
        assert _select(df) == "line"

    def test_selects_pie_for_small_proportion_data(self):
        """Single numeric col + single text col + <=6 rows should select pie."""
        _select = self._import_select()
        df = pd.DataFrame({"category": ["X", "Y", "Z"], "count": [10, 20, 30]})
        assert _select(df) == "pie"

    def test_selects_bar_for_no_numeric_cols(self):
        """Only non-numeric columns should default to bar."""
        _select = self._import_select()
        df = pd.DataFrame({"name": ["A", "B"], "desc": ["foo", "bar"]})
        assert _select(df) == "bar"

    def test_selects_bar_for_multiple_numeric_cols(self):
        """Multiple numeric columns without time label should select bar."""
        _select = self._import_select()
        df = pd.DataFrame({
            "item": ["A", "B"],
            "sales": [100, 200],
            "cost": [50, 80],
        })
        assert _select(df) == "bar"


class TestSampleDataframe:
    """Test _sample_dataframe."""

    def _import_sample(self):
        _reload_chart_agent()
        from agents.chart_agent import _sample_dataframe
        return _sample_dataframe

    def test_no_sampling_under_limit(self):
        _sample = self._import_sample()
        df = pd.DataFrame({"a": range(30)})
        assert len(_sample(df, max_rows=50)) == 30

    def test_sampling_over_limit(self):
        _sample = self._import_sample()
        df = pd.DataFrame({"a": range(100)})
        assert len(_sample(df, max_rows=50)) <= 50

    def test_empty_dataframe(self):
        _sample = self._import_sample()
        df = pd.DataFrame()
        assert len(_sample(df, max_rows=50)) == 0


class TestChartAgentNode:
    """Test chart_agent_node function."""

    def _import_node(self):
        _reload_chart_agent()
        from agents.chart_agent import chart_agent_node
        return chart_agent_node

    def test_empty_result_returns_empty_paths(self):
        node = self._import_node()
        assert node({"run_id": "test-1", "nl2sql_result": []})["chart_paths"] == []

    def test_skips_when_no_data(self):
        node = self._import_node()
        assert node({"run_id": "test-2"})["chart_paths"] == []

    def test_handles_malformed_data_gracefully(self):
        node = self._import_node()
        result = node({"run_id": "test-3", "nl2sql_result": [{"a": None}]})
        assert "chart_paths" in result
        assert isinstance(result["chart_paths"], list)
