import logging
from typing import Literal

from langgraph.graph import StateGraph, START, END
from langgraph.checkpoint.memory import MemorySaver
from opentelemetry import trace

from orchestrator.state import AgentState
from orchestrator.tracing import report_run_step
from agents.data_analysis_agent import data_analysis_node
from agents.report_generation_agent import report_generation_node

logger = logging.getLogger(__name__)
tracer = trace.get_tracer(__name__)

# Mock NL2SQL Node (to be replaced with actual grpc invocation or direct logic)
def nl2sql_node(state: AgentState) -> dict:
    with tracer.start_as_current_span("nl2sql_node"):
        logger.info("Executing NL2SQL Agent (Mock for Orchestrator)")
        run_id = state.get("run_id", "")
        report_run_step(run_id, "nl2sql_agent", "running", "Started NL2SQL node")
    
    # Since the system already has a Go -> Python NL2SQL gRPC flow, 
    # we can either reuse the same logic or let Go inject the nl2sql_result directly.
    # We will assume `nl2sql_result` is populated either before graph execution
    # or by this node executing the actual SQL.
    # For now, it's a pass-through if already populated.
    if not state.get("nl2sql_result"):
        # Dummy data if no result was provided
        report_run_step(run_id, "nl2sql_agent", "succeeded", output_summary="Mocked result")
        return {"nl2sql_result": [{"mock_col": 1, "value": 100}]}
        
    report_run_step(run_id, "nl2sql_agent", "succeeded", output_summary="Used existing result")
    return {}

def route_next(state: AgentState) -> Literal["analysis_node", "report_node", "__end__"]:
    # Simple rule-based routing based on user input intent
    user_input = state.get("user_input", "").lower()
    
    if "分析" in user_input or "analyze" in user_input or "trend" in user_input:
        return "analysis_node"
    elif "报告" in user_input or "report" in user_input or "export" in user_input:
        return "report_node"
    else:
        return "__end__"

def build_graph():
    builder = StateGraph(AgentState)
    
    # 6.2 Register Nodes
    builder.add_node("nl2sql_node", nl2sql_node)
    builder.add_node("analysis_node", data_analysis_node)
    builder.add_node("report_node", report_generation_node)
    
    # Define edges
    builder.add_edge(START, "nl2sql_node")
    
    # 6.3 Conditional Edges
    builder.add_conditional_edges("nl2sql_node", route_next)
    
    builder.add_edge("analysis_node", "report_node")
    builder.add_edge("report_node", END)
    
    # 6.4 Checkpointer
    memory = MemorySaver()
    
    graph = builder.compile(checkpointer=memory)
    return graph

# Global graph instance
workflow_graph = build_graph()
