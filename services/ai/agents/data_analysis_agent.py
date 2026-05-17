import pandas as pd
import numpy as np
import logging
import os
from openai import OpenAI
from orchestrator.state import AgentState
from orchestrator.tracing import report_run_step

logger = logging.getLogger(__name__)

def truncate_data(data: list[dict], max_rows: int = 500) -> list[dict]:
    if len(data) > max_rows:
        logger.warning(f"Data truncated from {len(data)} to {max_rows} rows to prevent context overflow.")
        return data[:max_rows]
    return data

def data_analysis_node(state: AgentState) -> dict:
    logger.info("Executing Data Analysis Agent")
    run_id = state.get("run_id", "")
    report_run_step(run_id, "data_analysis_agent", "running", "Started data analysis...")
    
    result_data = state.get("nl2sql_result", [])
    if not result_data:
        return {"analysis_summary": "No data available for analysis."}
        
    result_data = truncate_data(result_data)
    df = pd.DataFrame(result_data)
    
    numeric_cols = df.select_dtypes(include=[np.number]).columns
    summary_parts = []
    
    if len(numeric_cols) > 0:
        desc = df[numeric_cols].describe()
        summary_parts.append("Numeric Columns Summary:\n" + desc.to_string())
        
        for col in numeric_cols:
            mean = df[col].mean()
            std = df[col].std()
            if std > 0:
                anomalies = df[abs(df[col] - mean) > 3 * std]
                if not anomalies.empty:
                    summary_parts.append(f"Potential anomalies in {col}: {len(anomalies)} rows found > 3 std devs.")
    else:
        summary_parts.append("No numeric columns found for deep analysis.")
        
    raw_stats = "\n\n".join(summary_parts)
    
    # 调用 LLM 生成可读的业务摘要
    api_key = os.environ.get("OPENAI_API_KEY")
    if not api_key:
        return {"analysis_summary": raw_stats + "\n(LLM analysis skipped: missing API key)"}
        
    client = OpenAI(
        api_key=api_key,
        base_url=os.environ.get("OPENAI_BASE_URL", "https://api.openai.com/v1"),
    )
    
    try:
        r = client.chat.completions.create(
            model=os.environ.get("OPENAI_MODEL", "gpt-4o-mini"),
            messages=[
                {"role": "system", "content": "You are a data analyst. Write a concise business summary of the statistical findings."},
                {"role": "user", "content": f"User intent: {state.get('user_input', '')}\n\nStatistics:\n{raw_stats}"}
            ],
            temperature=0.3,
        )
        llm_summary = (r.choices[0].message.content or "").strip()
        report_run_step(run_id, "data_analysis_agent", "succeeded", "Calculated stats", llm_summary[:200])
        return {"analysis_summary": llm_summary}
    except Exception as e:
        logger.error(f"LLM analysis failed: {e}")
        report_run_step(run_id, "data_analysis_agent", "failed", "Calculated stats", "", str(e))
        return {"analysis_summary": raw_stats + f"\n(LLM analysis failed: {e})"}
