import { useState, type CSSProperties } from "react";
import { ResultTable } from "../ResultTable";
import { ChartView } from "./ChartView";
import type { ApiMessage, MessageContent, RunStep } from "../types/api";

interface MessageBlockProps {
  msg: ApiMessage;
}

export function MessageBlock({ msg }: MessageBlockProps) {
  const isUser = msg.role === "user";
  const bubble: CSSProperties = {
    alignSelf: isUser ? "flex-end" : "flex-start",
    maxWidth: "92%",
    padding: "10px 14px",
    borderRadius: 12,
    background: isUser ? "#1a5fb4" : "#eef1f5",
    color: isUser ? "#fff" : "#111",
    fontSize: 14,
  };

  return (
    <div style={bubble}>
      <div style={{ fontSize: 11, opacity: 0.75, marginBottom: 6 }}>
        {msg.role} · {msg.created_at}
      </div>
      <MessageBody content={msg.content} />
    </div>
  );
}

export function MessageBody({ content }: { content: MessageContent }) {
  if (content === null || content === undefined) {
    return <span style={{ opacity: 0.7 }}>（空）</span>;
  }
  if (typeof content !== "object") {
    return <pre style={{ margin: 0, whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{String(content)}</pre>;
  }
  const c = content as unknown as Record<string, unknown>;

  if (typeof c.text === "string") {
    return <p style={{ margin: 0, whiteSpace: "pre-wrap" }}>{c.text}</p>;
  }

  if (typeof c.error === "string") {
    return (
      <div>
        <strong>错误</strong>
        <pre style={{ margin: "8px 0 0", whiteSpace: "pre-wrap" }}>{c.error}</pre>
        {typeof c.code === "string" ? <code style={{ fontSize: 12 }}>{c.code}</code> : null}
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
        <strong style={{ fontSize: 12 }}>生成报告</strong>
        <pre style={{ margin: "8px 0", whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{finalReportText}</pre>
        {runId && (
          <div style={{ marginTop: 8 }}>
            <a href={`/api/v1/runs/${runId}/report`} target="_blank" rel="noreferrer" style={{
              display: "inline-block", padding: "4px 12px", background: "#10a37f", color: "#fff", textDecoration: "none", borderRadius: 4, fontSize: 13
            }}>下载 Excel 报告</a>
          </div>
        )}
      </div>
    );
  }

  return (
    <pre style={{ margin: 0, fontSize: 12, overflowX: "auto" }}>{JSON.stringify(content, null, 2)}</pre>
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
      <div style={{ marginBottom: 8 }}>
        <strong style={{ fontSize: 12 }}>SQL</strong>
        <pre
          style={{
            margin: "4px 0 0",
            padding: 8,
            background: "rgba(0,0,0,0.06)",
            borderRadius: 6,
            fontSize: 12,
            overflowX: "auto",
          }}
        >
          {sql}
        </pre>
      </div>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 4 }}>
        <strong style={{ fontSize: 12 }}>结果</strong>
        {hasNumeric && (
          <button
            type="button"
            onClick={() => setView(v => v === "table" ? "chart" : "table")}
            style={{
              padding: "2px 10px",
              fontSize: 11,
              cursor: "pointer",
              border: "1px solid #ccc",
              borderRadius: 4,
              background: "#fff",
            }}
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
        <p style={{ fontSize: 12, margin: "8px 0 0", opacity: 0.85 }}>
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
    <div style={{ padding: 12, background: "#f8f9fa", borderRadius: 8, fontSize: 13, border: "1px dashed #ccc" }}>
      <strong>中间执行步骤:</strong>
      <ul style={{ paddingLeft: 20, margin: "8px 0 0" }}>
        {steps.map((step, idx) => (
          <li key={idx}>
            <span style={{color: step.status === 'running' ? '#10a37f' : step.status === 'failed' ? 'red' : '#555'}}>
              [{step.agent_name}]
            </span>{" "}
            {step.status}: {step.output_summary || step.input_summary}
            {step.error_message && <span style={{color:'red'}}> (Error: {step.error_message})</span>}
          </li>
        ))}
      </ul>
    </div>
  );
}
