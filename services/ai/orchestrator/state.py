from typing import TypedDict, Any, Optional

class AgentState(TypedDict, total=False):
    run_id: str
    user_input: str
    schema_context: str
    rag_context: str
    sql: str
    nl2sql_result: list[dict[str, Any]]
    analysis_summary: str
    final_report: str
    error: str
    chart_paths: list[str]
    nl2sql_sql: str
    nl2sql_error: str
    workflow: str
