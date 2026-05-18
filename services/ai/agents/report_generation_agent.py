import logging
import os
import re
import uuid
import hashlib
import pandas as pd
from datetime import datetime
from orchestrator.state import AgentState
from orchestrator.tracing import report_run_step

logger = logging.getLogger(__name__)

# 报告输出目录（环境变量化）
REPORT_OUTPUT_DIR = os.getenv("REPORT_OUTPUT_DIR", "/tmp/reports")


def report_generation_node(state: AgentState) -> dict:
    logger.info("Executing Report Generation Agent")
    run_id = state.get("run_id", "")
    report_run_step(run_id, "report_generation_agent", "running", "Generating report...")

    # ----------------------------------------------------------------
    # 路径遍历防护：确保 run_id 不包含路径分隔符
    # ----------------------------------------------------------------
    raw_run_id = state.get("run_id") or str(uuid.uuid4())
    if not re.match(r'^[a-fA-F0-9-]{36}$', raw_run_id):
        safe_run_id = hashlib.sha256(raw_run_id.encode()).hexdigest()[:16]
    else:
        safe_run_id = raw_run_id

    # 生成 Markdown 报告
    md_lines = [
        "# Data Analysis Report",
        f"Generated at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
        f"\n## Request\n{state.get('user_input', 'N/A')}",
        f"\n## Analysis Summary\n{state.get('analysis_summary', 'No summary available.')}",
        f"\n## Data Extract",
    ]

    result_data = state.get("nl2sql_result", [])
    if result_data:
        df = pd.DataFrame(result_data)
        md_lines.append(df.head(10).to_markdown(index=False))
        if len(result_data) > 10:
            md_lines.append(f"\n*(Showing top 10 rows of {len(result_data)} total)*")
    else:
        md_lines.append("*No data returned.*")

    # 如果存在图表路径，添加图表可视化章节
    chart_paths = state.get("chart_paths", [])
    if chart_paths:
        md_lines.append("\n## 数据可视化")
        for p in chart_paths:
            filename = os.path.basename(p)
            md_lines.append(f"\n![chart](./{filename})")
        md_lines.append(f"\n*共 {len(chart_paths)} 个图表*")

    final_md = "\n".join(md_lines)

    # 保存到 report 目录（环境变量化）
    os.makedirs(REPORT_OUTPUT_DIR, exist_ok=True)

    if result_data:
        excel_path = os.path.join(REPORT_OUTPUT_DIR, f"{safe_run_id}.xlsx")
        df.to_excel(excel_path, index=False)
        logger.info("Excel report saved to %s", excel_path)

    md_path = os.path.join(REPORT_OUTPUT_DIR, f"{safe_run_id}.md")
    with open(md_path, "w", encoding="utf-8") as f:
        f.write(final_md)

    report_run_step(run_id, "report_generation_agent", "succeeded", "Report generated", f"Files saved for run_id: {safe_run_id}")
    return {"final_report": final_md}
