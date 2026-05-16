import logging
import os
import numpy as np
import pandas as pd
import matplotlib
matplotlib.use("Agg")  # Non-interactive backend for server environments
import matplotlib.pyplot as plt
from matplotlib import font_manager
from orchestrator.state import AgentState
from orchestrator.tracing import report_run_step

logger = logging.getLogger(__name__)

# Max data points before sampling
MAX_CHART_POINTS = 50

# Chart output directory
CHART_OUTPUT_DIR = "/tmp/reports"


def _sample_dataframe(df: pd.DataFrame, max_rows: int = MAX_CHART_POINTS) -> pd.DataFrame:
    """Sample dataframe to max_rows by taking evenly spaced rows."""
    if len(df) <= max_rows:
        return df
    step = max(len(df) // max_rows, 1)
    return df.iloc[::step].head(max_rows)


def _configure_chinese_fonts():
    """Try to find and configure a CJK font for matplotlib."""
    # Try common CJK fonts in order of preference
    candidates = [
        "Noto Sans CJK SC", "Noto Sans CJK TC", "WenQuanYi Micro Hei",
        "SimHei", "Microsoft YaHei", "PingFang SC", "STHeiti",
        "AR PL UMing CN", "AR PL UKai CN",
    ]

    available = {f.name for f in font_manager.fontManager.ttflist}
    for name in candidates:
        if name in available:
            plt.rcParams["font.sans-serif"] = [name, "DejaVu Sans"]
            plt.rcParams["axes.unicode_minus"] = False
            logger.debug(f"Matplotlib using CJK font: {name}")
            return

    # Fallback: scan system font dirs for any CJK font
    try:
        for font_dir in ["/usr/share/fonts", "/usr/local/share/fonts"]:
            if os.path.isdir(font_dir):
                for root, _dirs, files in os.walk(font_dir):
                    for f in files:
                        if f.endswith((".ttf", ".otf")) and any(
                            kw in f.lower() for kw in ["noto", "cjk", "chinese", "wenquan", "simhei", "wqy"]
                        ):
                            font_manager.fontManager.addfont(os.path.join(root, f))
                            logger.info(f"Added font from system: {f}")
    except Exception as e:
        logger.warning(f"Font scan failed: {e}")

    plt.rcParams["font.sans-serif"] = ["DejaVu Sans"]
    plt.rcParams["axes.unicode_minus"] = False
    logger.warning("No CJK font found; chart labels may show tofu (□). Install fonts-noto-cjk for CJK support.")


# Configure fonts at import time
_configure_chinese_fonts()


def _select_chart_type(df: pd.DataFrame) -> str:
    """Auto-select chart type based on column types.

    Returns one of: 'bar', 'line', 'pie'
    """
    numeric_cols = list(df.select_dtypes(include=[np.number]).columns)
    non_numeric_cols = [c for c in df.columns if c not in numeric_cols]

    if len(numeric_cols) == 0:
        return "bar"  # Default, will likely fail gracefully

    # Pie chart: 1 text col + 1 numeric col, <= 6 categories
    if len(numeric_cols) == 1 and len(non_numeric_cols) >= 1:
        if len(df) <= 6:
            return "pie"

    # Line chart: check if first non-numeric column looks like time/date
    if len(non_numeric_cols) >= 1:
        first_label_col = non_numeric_cols[0]
        sample_vals = df[first_label_col].dropna().head(3).astype(str)
        time_patterns = ["年", "月", "日", "-", "/", ":", "Q", "W"]
        if any(any(p in str(v) for p in time_patterns) for v in sample_vals):
            return "line"

    # If we have multiple numeric columns or a single text label + numeric, use bar
    if (len(numeric_cols) >= 1 and len(non_numeric_cols) >= 1) or len(numeric_cols) >= 2:
        return "bar"

    # Default: bar chart
    return "bar"


def _get_label_and_value_cols(df: pd.DataFrame) -> tuple[list[str], list[str]]:
    """Extract label columns (non-numeric) and value columns (numeric)."""
    numeric = list(df.select_dtypes(include=[np.number]).columns)
    non_numeric = [c for c in df.columns if c not in numeric]
    return non_numeric, numeric


def _draw_bar_chart(df: pd.DataFrame, run_id: str, idx: int) -> str | None:
    """Generate a bar chart PNG. Returns file path or None on failure."""
    non_num, num = _get_label_and_value_cols(df)
    if not num:
        logger.warning("Bar chart: no numeric columns found")
        return None

    labels = df[non_num[0]].astype(str).tolist() if non_num else [str(i) for i in range(len(df))]

    fig, ax = plt.subplots(figsize=(10, 5))
    x = range(len(labels))
    width = 0.8 / max(len(num), 1)

    for i, col in enumerate(num):
        ax.bar([p + i * width for p in x], df[col].values, width, label=col)

    ax.set_xticks([p + width * (len(num) - 1) / 2 for p in x])
    ax.set_xticklabels(labels, rotation=30, ha="right", fontsize=9)
    ax.legend(fontsize=9)
    ax.set_title(f"Run {run_id[:8]}", fontsize=11)

    os.makedirs(CHART_OUTPUT_DIR, exist_ok=True)
    path = f"{CHART_OUTPUT_DIR}/{run_id}_chart_{idx}.png"
    fig.tight_layout()
    fig.savefig(path, dpi=100)
    plt.close(fig)
    logger.info(f"Bar chart saved: {path}")
    return path


def _draw_line_chart(df: pd.DataFrame, run_id: str, idx: int) -> str | None:
    """Generate a line chart PNG. Returns file path or None on failure."""
    non_num, num = _get_label_and_value_cols(df)
    if not num:
        logger.warning("Line chart: no numeric columns found")
        return None

    labels = df[non_num[0]].astype(str).tolist() if non_num else [str(i) for i in range(len(df))]

    fig, ax = plt.subplots(figsize=(10, 5))
    for col in num:
        ax.plot(labels, df[col].values, marker="o", linewidth=2, label=col)

    ax.set_xticklabels(labels, rotation=30, ha="right", fontsize=9)
    ax.legend(fontsize=9)
    ax.grid(True, alpha=0.3)
    ax.set_title(f"Run {run_id[:8]}", fontsize=11)

    os.makedirs(CHART_OUTPUT_DIR, exist_ok=True)
    path = f"{CHART_OUTPUT_DIR}/{run_id}_chart_{idx}.png"
    fig.tight_layout()
    fig.savefig(path, dpi=100)
    plt.close(fig)
    logger.info(f"Line chart saved: {path}")
    return path


def _draw_pie_chart(df: pd.DataFrame, run_id: str, idx: int) -> str | None:
    """Generate a pie chart PNG. Returns file path or None on failure."""
    non_num, num = _get_label_and_value_cols(df)
    if not num or not non_num:
        logger.warning("Pie chart: need 1 label col + 1 value col")
        return None

    labels = df[non_num[0]].astype(str).tolist()
    values = df[num[0]].values

    fig, ax = plt.subplots(figsize=(7, 7))
    wedges, texts, autotexts = ax.pie(
        values,
        labels=labels,
        autopct=lambda pct: f"{pct:.1f}%" if pct > 3 else "",
        startangle=90,
        textprops={"fontsize": 9},
    )

    ax.set_title(f"Run {run_id[:8]}", fontsize=11)

    os.makedirs(CHART_OUTPUT_DIR, exist_ok=True)
    path = f"{CHART_OUTPUT_DIR}/{run_id}_chart_{idx}.png"
    fig.tight_layout()
    fig.savefig(path, dpi=100)
    plt.close(fig)
    logger.info(f"Pie chart saved: {path}")
    return path


def chart_agent_node(state: AgentState) -> dict:
    """Chart Agent node for LangGraph.

    Reads nl2sql_result from state, auto-selects chart type,
    generates PNG chart(s), and returns chart_paths.
    On failure: returns empty chart_paths and logs warning.
    """
    run_id = state.get("run_id", "unknown")
    report_run_step(run_id, "chart_agent", "running", "Generating data charts...")

    result_data = state.get("nl2sql_result", [])
    chart_paths: list[str] = []

    if not result_data:
        logger.info("Chart agent: no nl2sql_result data, skipping")
        report_run_step(run_id, "chart_agent", "skipped", "No data to chart")
        return {"chart_paths": chart_paths}

    try:
        df = pd.DataFrame(result_data)

        # Sample large datasets
        if len(df) > MAX_CHART_POINTS:
            logger.info(f"Chart agent: sampling {len(df)} rows to {MAX_CHART_POINTS}")
            df = _sample_dataframe(df)

        chart_type = _select_chart_type(df)
        logger.info(f"Chart agent: selected chart type '{chart_type}' for {len(df)} rows")

        chart_idx = 0
        if chart_type == "bar":
            path = _draw_bar_chart(df, run_id, chart_idx)
            if path:
                chart_paths.append(path)
        elif chart_type == "line":
            path = _draw_line_chart(df, run_id, chart_idx)
            if path:
                chart_paths.append(path)
        elif chart_type == "pie":
            path = _draw_pie_chart(df, run_id, chart_idx)
            if path:
                chart_paths.append(path)

        # Also generate bar chart as companion when line or pie selected (if 2+ numeric cols)
        if chart_type == "line" and len(df.select_dtypes(include=[np.number]).columns) >= 2:
            chart_idx += 1
            path = _draw_bar_chart(df, run_id, chart_idx)
            if path:
                chart_paths.append(path)

        summary = f"Generated {len(chart_paths)} chart(s)" if chart_paths else "No charts generated"
        report_run_step(run_id, "chart_agent", "succeeded", summary)

    except Exception as e:
        logger.warning(f"Chart agent failed (non-blocking): {e}", exc_info=True)
        report_run_step(run_id, "chart_agent", "failed", error_message=str(e))
        chart_paths = []

    return {"chart_paths": chart_paths}
