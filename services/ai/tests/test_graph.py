"""Tests for orchestrator/graph.py"""

import sys
import pytest
from unittest.mock import patch, MagicMock

# Only mock the missing sqlite submodule
sys.modules["langgraph.checkpoint.sqlite"] = MagicMock()


class TestRouteFunctions:
    """Test pure route functions from graph module."""

    def _import(self):
        """Import graph module with external deps properly handled."""
        if "orchestrator.graph" in sys.modules:
            del sys.modules["orchestrator.graph"]

        with patch("orchestrator.graph.SqliteSaver"), \
             patch("orchestrator.graph.os.makedirs"), \
             patch("orchestrator.graph.os.getenv", return_value=":memory:"):
            import orchestrator.graph as g
            return g

    # ---- route_next ----
    def test_route_next_simple_workflow_ends(self):
        g = self._import()
        assert g.route_next({"user_input": "hello", "workflow": "simple"}) == "__end__"

    def test_route_next_agent_pipeline_goes_to_analysis(self):
        g = self._import()
        assert g.route_next({"user_input": "hello", "workflow": "agent_pipeline"}) == "analysis_node"

    def test_route_next_analyze_keyword_goes_to_analysis(self):
        g = self._import()
        assert g.route_next({"user_input": "分析一下销售数据", "workflow": "auto"}) == "analysis_node"

    def test_route_next_english_analyze_keyword(self):
        g = self._import()
        assert g.route_next({"user_input": "analyze sales data", "workflow": "auto"}) == "analysis_node"

    def test_route_next_chart_keyword(self):
        g = self._import()
        assert g.route_next({"user_input": "画一个图表", "workflow": "auto"}) == "chart_node"

    def test_route_next_report_keyword(self):
        g = self._import()
        assert g.route_next({"user_input": "生成报告", "workflow": "auto"}) == "report_node"

    def test_route_next_default_ends(self):
        g = self._import()
        assert g.route_next({"user_input": "show me the data", "workflow": "auto"}) == "__end__"

    # ---- route_after_analysis ----
    def test_route_after_analysis_chart_for_pipeline(self):
        g = self._import()
        assert g.route_after_analysis({"workflow": "agent_pipeline"}) == "chart_node"

    def test_route_after_analysis_report_for_auto(self):
        g = self._import()
        assert g.route_after_analysis({"workflow": "auto"}) == "report_node"

    # ---- route_after_chart ----
    def test_route_after_chart_report_for_pipeline(self):
        g = self._import()
        assert g.route_after_chart({"workflow": "agent_pipeline"}) == "report_node"

    def test_route_after_chart_ends_for_auto(self):
        g = self._import()
        assert g.route_after_chart({"workflow": "auto"}) == "__end__"


class TestGraphStructure:
    """Test graph structure."""

    def test_graph_has_correct_nodes(self):
        """Verify build_graph creates correct node structure."""
        import orchestrator.graph as g

        mock_builder = MagicMock()
        mock_builder.nodes = {}
        mock_sg_cls = MagicMock(return_value=mock_builder)

        with patch.object(g, 'StateGraph', mock_sg_cls), \
             patch.object(g, 'SqliteSaver'), \
             patch.object(g, 'nl2sql_node'), \
             patch.object(g, 'data_analysis_node'), \
             patch.object(g, 'chart_agent_node'), \
             patch.object(g, 'report_generation_node'), \
             patch.object(g, 'START', "__start__"), \
             patch.object(g, 'END', "__end__"):
            g.build_graph()

            node_names = [c[0][0] for c in mock_builder.add_node.call_args_list]
            assert "nl2sql_node" in node_names
            assert "analysis_node" in node_names
            assert "chart_node" in node_names
            assert "report_node" in node_names
