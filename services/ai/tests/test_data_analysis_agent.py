import pytest
import pandas as pd
import numpy as np
from unittest.mock import patch, MagicMock
import sys
import os

# 添加父目录到路径以允许导入
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
        """当 nl2sql_result 为空时，返回适当的消息。"""
        state = {
            "run_id": "test-run-1",
            "user_input": "test query",
            "nl2sql_result": [],
        }
        with patch("agents.data_analysis_agent.report_run_step"):
            result = data_analysis_node(state)
        assert "No data available" in result["analysis_summary"]

    def test_basic_stats(self):
        """含有数值数据时，统计信息应包含描述性输出。"""
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
        # 应包含数值分析（无 OpenAI 密钥时为原始统计信息）
        assert "Numeric Columns Summary" in summary or "std" in summary.lower() or "mean" in summary.lower() or "LLM" in summary

    def test_no_numeric_columns(self):
        """非数值数据也应生成消息。"""
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
