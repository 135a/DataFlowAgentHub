from __future__ import annotations
import asyncio
import json
import logging
import os
from typing import Literal

from langgraph.graph import StateGraph, START, END
from langgraph.checkpoint.sqlite import SqliteSaver
from opentelemetry import trace

from orchestrator.state import AgentState
from orchestrator.tracing import report_run_step
from hub_ai._client import get_client_sync
from agents.data_analysis_agent import data_analysis_node
from agents.report_generation_agent import report_generation_node
from agents.chart_agent import chart_agent_node

logger = logging.getLogger(__name__)
tracer = trace.get_tracer(__name__)


def nl2sql_node(state: AgentState) -> dict:
    """通过 gRPC 调用 Go API 的 InternalNL2SQL 执行 NL2SQL。

    同步函数，因为在 LangGraph invoke 中运行（同步线程池）。
    执行完成后会尝试 RAG 知识检索，结果写入 rag_context。
    """
    with tracer.start_as_current_span("nl2sql_node"):
        run_id = state.get("run_id", "")
        report_run_step(run_id, "nl2sql_agent", "running", "Calling NL2SQL via Go API gRPC")

        user_input = state.get("user_input", "")
        schema_context = state.get("schema_context", "{}")
        result: dict = {}

        try:
            client = get_client_sync()
            nl2sql_result = client.internal_nl2sql_sync(
                user_message=user_input,
                schema_json=schema_context,
                dialect="postgres",
            )

            if not nl2sql_result.get("ok", True):
                error_msg = nl2sql_result.get("error_message", "unknown error")
                logger.error("NL2SQL via gRPC failed: %s", error_msg)
                report_run_step(
                    run_id, "nl2sql_agent", "failed",
                    error_message=error_msg,
                )
                result["nl2sql_error"] = error_msg
            else:
                rows = nl2sql_result.get("rows", [])
                sql = nl2sql_result.get("sql", "")
                logger.info("NL2SQL returned %d rows, SQL: %s", len(rows), sql[:200])
                report_run_step(
                    run_id, "nl2sql_agent", "succeeded",
                    output_summary=f"Generated SQL, returned {len(rows)} rows",
                )
                result["nl2sql_result"] = rows
                result["nl2sql_sql"] = sql

        except Exception as e:
            logger.error("NL2SQL node failed: %s", e)
            report_run_step(
                run_id, "nl2sql_agent", "failed",
                error_message=str(e),
            )
            result["nl2sql_error"] = str(e)

        # ------------------------------------------------------------------
        # RAG 知识检索：填充 rag_context 供下游 agent 使用
        # ------------------------------------------------------------------
        workspace_id = state.get("workspace_id")
        if workspace_id and not state.get("rag_context"):
            try:
                from rag.knowledge_base import KnowledgeBase
                kb = KnowledgeBase(workspace_id)
                rag_snippets = kb.search(user_input, top_k=3)
                if rag_snippets and isinstance(rag_snippets, list):
                    context_parts = []
                    for i, snippet in enumerate(rag_snippets):
                        content = snippet.get("content", "")
                        if content:
                            context_parts.append(f"[{i+1}] {content}")
                    if context_parts:
                        result["rag_context"] = "\n".join(context_parts)
                        logger.info("RAG context added: %d documents", len(context_parts))
            except Exception as e:
                logger.warning("RAG search failed (non-blocking): %s", e)

        return result


def route_next(state: AgentState) -> Literal["analysis_node", "chart_node", "report_node", "__end__"]:
    """根据用户输入关键词和工作流参数路由到下一个节点。"""
    user_input = state.get("user_input", "").lower()
    workflow = state.get("workflow", "auto")

    # 显式工作流参数优先
    if workflow == "simple":
        return "__end__"

    if workflow == "agent_pipeline":
        return "analysis_node"

    # 自动：基于关键词的路由（支持中文和英文）
    chart_kw = ("chart", "图表", "可视化", "plot", "graph")
    analyze_kw = ("分析", "analyze", "trend", "趋势", "对比", "compare")
    report_kw = ("报告", "report", "export", "导出", "简报")

    if any(kw in user_input for kw in chart_kw):
        return "chart_node"
    elif any(kw in user_input for kw in analyze_kw):
        return "analysis_node"
    elif any(kw in user_input for kw in report_kw):
        return "report_node"

    # 默认：在 NL2SQL 后结束（无需更多 agent）
    return "__end__"


def route_after_analysis(state: AgentState) -> Literal["chart_node", "report_node"]:
    """分析后：如果是 agent_pipeline 则路由到 chart_node，否则直接到 report。"""
    workflow = state.get("workflow", "auto")
    if workflow == "agent_pipeline":
        return "chart_node"
    return "report_node"


def route_after_chart(state: AgentState) -> Literal["report_node", "__end__"]:
    """图表后：如果不是仅报告模式，则路由到 report_node。"""
    workflow = state.get("workflow", "auto")
    if workflow == "agent_pipeline":
        return "report_node"
    return "__end__"


def build_graph():
    builder = StateGraph(AgentState)

    builder.add_node("nl2sql_node", nl2sql_node)
    builder.add_node("analysis_node", data_analysis_node)
    builder.add_node("chart_node", chart_agent_node)
    builder.add_node("report_node", report_generation_node)

    builder.add_edge(START, "nl2sql_node")
    builder.add_conditional_edges("nl2sql_node", route_next)
    builder.add_conditional_edges("analysis_node", route_after_analysis)
    builder.add_conditional_edges("chart_node", route_after_chart)
    builder.add_edge("report_node", END)

    # 环境变量化 + :memory: 特殊处理
    db_path = os.getenv("LANGGRAPH_DB_PATH", "/data/langgraph/checkpoints.db")
    if db_path != ":memory:":
        os.makedirs(os.path.dirname(db_path), exist_ok=True)
    else:
        db_path = ":memory:"
    checkpointer = SqliteSaver.from_conn_string(db_path)
    graph = builder.compile(checkpointer=checkpointer)
    return graph


# 全局图实例
workflow_graph = build_graph()
