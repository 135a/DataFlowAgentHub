import logging
import os
import uuid
import pandas as pd
from datetime import datetime
from orchestrator.state import AgentState
from orchestrator.tracing import report_run_step

logger = logging.getLogger(__name__)

def report_generation_node(state: AgentState) -> dict:
    logger.info("Executing Report Generation Agent")
    run_id = state.get("run_id", "")
    report_run_step(run_id, "report_generation_agent", "running", "Generating report...")
    
    # 生成 Markdown 报告
    md_lines = [
        f"# Data Analysis Report",
        f"Generated at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
        f"\n## Request\n{state.get('user_input', 'N/A')}",
        f"\n## Analysis Summary\n{state.get('analysis_summary', 'No summary available.')}",
        f"\n## Data Extract"
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

    # 生成 Excel（保存到本地挂载或临时目录）
    # 写入 /tmp 目录，该目录可以是 Docker 中的卷挂载
    os.makedirs("/tmp/reports", exist_ok=True)
    report_id = state.get('run_id')
    if not report_id:
        report_id = str(uuid.uuid4())
    
    if result_data:
        excel_path = f"/tmp/reports/{report_id}.xlsx"
        df.to_excel(excel_path, index=False)
        logger.info(f"Excel report saved to {excel_path}")
        
    md_path = f"/tmp/reports/{report_id}.md"
    with open(md_path, "w", encoding="utf-8") as f:
        f.write(final_md)
        
    report_run_step(run_id, "report_generation_agent", "succeeded", "Report generated", f"Files saved for report_id: {report_id}")
    return {"final_report": final_md}
