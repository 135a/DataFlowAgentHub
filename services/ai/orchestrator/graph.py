import httpx
import json
import logging
import os
from typing import Literal

from langgraph.graph import StateGraph, START, END
from langgraph.checkpoint.memory import MemorySaver
from opentelemetry import trace

from orchestrator.state import AgentState
from orchestrator.tracing import report_run_step
from hub_ai.shared import make_headers
from agents.data_analysis_agent import data_analysis_node
from agents.report_generation_agent import report_generation_node

logger = logging.getLogger(__name__)
tracer = trace.get_tracer(__name__)


def nl2sql_node(state: AgentState) -> dict:
    """Execute NL2SQL by calling Go API's /internal/nl2sql endpoint."""
    with tracer.start_as_current_span("nl2sql_node"):
        run_id = state.get("run_id", "")
        report_run_step(run_id, "nl2sql_agent", "running", "Calling NL2SQL via Go API")

        user_input = state.get("user_input", "")
        schema_context = state.get("schema_context", "{}")

        api_url = os.environ.get("HUB_API_INTERNAL_URL", "http://api:8080")
        secret = os.environ.get("HUB_INTERNAL_HMAC_SECRET", "dev-hmac-secret-change-me")

        try:
            body = {
                "user_message": user_input,
                "schema_json": schema_context,
                "dialect": "postgres",
            }
            body_bytes = json.dumps(body).encode()
            headers = make_headers(secret, body_bytes)

            with httpx.Client(timeout=30.0) as client:
                resp = client.post(
                    f"{api_url}/internal/nl2sql",
                    headers=headers,
                    content=body_bytes,
                )
                resp.raise_for_status()
                result = resp.json()
                rows = result.get("rows", [])
                sql = result.get("sql", "")
                logger.info(f"NL2SQL returned {len(rows)} rows, SQL: {sql[:200]}")
                report_run_step(
                    run_id, "nl2sql_agent", "succeeded",
                    output_summary=f"Generated SQL, returned {len(rows)} rows"
                )
                return {"nl2sql_result": rows, "nl2sql_sql": sql}

        except Exception as e:
            logger.error(f"NL2SQL node failed: {e}")
            report_run_step(
                run_id, "nl2sql_agent", "failed",
                error_message=str(e)
            )
            return {"nl2sql_error": str(e)}


def route_next(state: AgentState) -> Literal["analysis_node", "report_node", "__end__"]:
    """Route to next node based on user input keywords and workflow parameter."""
    user_input = state.get("user_input", "").lower()
    workflow = state.get("workflow", "auto")

    # Explicit workflow parameter takes priority
    if workflow == "simple":
        return "__end__"

    if workflow == "agent_pipeline":
        return "analysis_node"

    # Auto: keyword-based routing (supports both Chinese and English)
    analyze_kw = ("分析", "analyze", "trend", "趋势", "对比", "compare")
    report_kw = ("报告", "report", "export", "导出", "简报")

    if any(kw in user_input for kw in analyze_kw):
        return "analysis_node"
    elif any(kw in user_input for kw in report_kw):
        return "report_node"

    # Default: end after nl2sql (no further agents needed)
    return "__end__"


def build_graph():
    builder = StateGraph(AgentState)

    builder.add_node("nl2sql_node", nl2sql_node)
    builder.add_node("analysis_node", data_analysis_node)
    builder.add_node("report_node", report_generation_node)

    builder.add_edge(START, "nl2sql_node")
    builder.add_conditional_edges("nl2sql_node", route_next)
    builder.add_edge("analysis_node", "report_node")
    builder.add_edge("report_node", END)

    memory = MemorySaver()
    graph = builder.compile(checkpointer=memory)
    return graph


# Global graph instance
workflow_graph = build_graph()
