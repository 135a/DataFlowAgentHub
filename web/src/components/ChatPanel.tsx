import { useState } from "react";
import { ResultTable } from "../ResultTable";
import { ChartView } from "./ChartView";
import type { ApiMessage, MessageContent, RunStep } from "../types/api";
import styles from "./ChatPanel.module.css";

interface MessageBlockProps {
  msg: ApiMessage;
}

export function MessageBlock({ msg }: MessageBlockProps) {
  const isUser = msg.role === "user";
  const bubbleClass = isUser
    ? `${styles.bubble} ${styles.bubbleUser}`
    : `${styles.bubble} ${styles.bubbleAssistant}`;

  return (
    <div className={bubbleClass}>
      <div className={styles.meta}>
        {msg.role} · {msg.created_at}
      </div>
      <MessageBody content={msg.content} />
    </div>
  );
}

export function MessageBody({ content }: { content: MessageContent }) {
  if (content === null || content === undefined) {
    return <span className={styles.emptyText}>（空）</span>;
  }
  if (typeof content !== "object") {
    return <pre className={styles.preWrap}>{String(content)}</pre>;
  }
  const c = content as unknown as Record<string, unknown>;

  if (typeof c.text === "string") {
    return <p className={styles.plainText}>{c.text}</p>;
  }

  if (typeof c.error === "string") {
    return (
      <div className={styles.errorBlock}>
        <strong>错误</strong>
        <pre className={styles.errorPre}>{c.error}</pre>
        {typeof c.code === "string" ? <code className={styles.errorCode}>{c.code}</code> : null}
      </div>
    );
  }

  const sql = typeof c.sql === "string" ? c.sql : null;
  const rawRows = c.rows;
  const rows = Array.isArray(rawRows) ? (rawRows as Record<string, unknown>[]) : null;

  if (sql !== null && rows) {
    const notes = c.notes;
    const hasNumeric = rows.length > 0 && Object.values(rows[0] || {}).some(v => typeof v === "number");
    return <SqlResultBlock sql={sql} rows={rows} hasNumeric={hasNumeric} notes={notes} />;
  }

  if (c.final_report) {
    const reportObj = c.final_report as Record<string, unknown>;
    const finalReportText = String(reportObj.final_report || "");
    const runId = c.run_id as string | undefined;
    return (
      <div>
        <strong className={styles.reportBlock}>生成报告</strong>
        <pre className={styles.reportPre}>{finalReportText}</pre>
        {runId && (
          <div style={{ marginTop: 8 }}>
            <a href={`/api/v1/runs/${runId}/report`} target="_blank" rel="noreferrer" className={styles.downloadLink}>
              下载 Excel 报告
            </a>
          </div>
        )}
      </div>
    );
  }

  return (
    <pre className={styles.jsonPre}>{JSON.stringify(content, null, 2)}</pre>
  );
}

function SqlResultBlock({ sql, rows, hasNumeric, notes }: {
  sql: string;
  rows: Record<string, unknown>[];
  hasNumeric: boolean;
  notes: unknown;
}) {
  const [view, setView] = useState<"table" | "chart">("table");

  return (
    <div>
      <div className={styles.sqlBlock}>
        <strong className={styles.sqlHeader}>SQL</strong>
        <pre className={styles.sqlPre}>
          {sql}
        </pre>
      </div>
      <div className={styles.resultHeader}>
        <strong className={styles.resultLabel}>结果</strong>
        {hasNumeric && (
          <button
            type="button"
            onClick={() => setView(v => v === "table" ? "chart" : "table")}
            className={styles.toggleBtn}
          >
            {view === "table" ? "图表" : "表格"}
          </button>
        )}
      </div>
      {view === "table" ? (
        <ResultTable rows={rows} />
      ) : (
        <ChartView rows={rows} />
      )}
      {notes != null &&
      (Array.isArray(notes) ? notes.length > 0 : String(notes).length > 0) ? (
        <p className={styles.notes}>
          自检：{Array.isArray(notes) ? notes.map(String).join("；") : String(notes)}
        </p>
      ) : null}
    </div>
  );
}

interface RunStepsPanelProps {
  steps: RunStep[];
}

export function RunStepsPanel({ steps }: RunStepsPanelProps) {
  if (steps.length === 0) return null;
  return (
    <div className={styles.runStepsPanel}>
      <strong>中间执行步骤:</strong>
      <ul className={styles.runStepsList}>
        {steps.map((step, idx) => (
          <li key={idx}>
            <span className={
              step.status === 'running' ? styles.stepRunning :
              step.status === 'failed' ? styles.stepFailed :
              styles.stepAgent
            }>
              [{step.agent_name}]
            </span>{" "}
            {step.status}: {step.output_summary || step.input_summary}
            {step.error_message && <span className={styles.stepErrorText}> (Error: {step.error_message})</span>}
          </li>
        ))}
      </ul>
    </div>
  );
}
