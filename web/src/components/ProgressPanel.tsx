import { useEffect, useRef, type CSSProperties } from "react";

export interface StepDef {
  name: string;
  weight: number;
}

export interface StepState {
  status: "waiting" | "running" | "completed" | "error";
  durationMs: number;
}

interface ProgressPanelProps {
  steps: StepDef[];
  stepStates: StepState[];
  elapsedMs: number;
  estimatedRemainingMs: number | null;
}

function fmt(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function fmtRange(ms: number): string {
  const low = Math.max(1, Math.round((ms * 0.7) / 1000));
  const high = Math.round((ms * 1.3) / 1000);
  if (low === high) return `~${low}s`;
  return `${low}-${high}s`;
}

const STATUS_ICON: Record<string, string> = {
  waiting: "⏳",
  running: "🔄",
  completed: "✅",
  error: "❌",
};

export function ProgressPanel({
  steps,
  stepStates,
  elapsedMs,
  estimatedRemainingMs,
}: ProgressPanelProps) {
  const completedWeight = stepStates.reduce(
    (s, st, i) => (st.status === "completed" ? s + steps[i].weight : s),
    0,
  );
  const totalProgress = stepStates.reduce((s, st, i) => {
    if (st.status === "completed") return s + steps[i].weight * 100;
    if (st.status === "running") {
      const def = steps[i];
      const pct = Math.min(95, (st.durationMs / (def.weight * 15000)) * 100);
      return s + def.weight * pct;
    }
    return s;
  }, 0);

  const container: CSSProperties = {
    marginTop: 12,
    padding: "12px 16px",
    background: "#f8f9fa",
    borderRadius: 10,
    border: "1px solid #e5e7eb",
    fontSize: 13,
  };
  const headerRow: CSSProperties = {
    display: "flex",
    justifyContent: "space-between",
    marginBottom: 10,
    color: "#374151",
  };

  return (
    <div style={container}>
      <div style={headerRow}>
        <strong>处理进度</strong>
        <span>
          已用 {fmt(elapsedMs)}
          {estimatedRemainingMs !== null && (
            <> · 预计剩余 {fmtRange(estimatedRemainingMs)}</>
          )}
        </span>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
        {steps.map((step, i) => {
          const st = stepStates[i];
          const pct =
            st.status === "completed"
              ? 100
              : st.status === "running"
                ? Math.min(95, (st.durationMs / (step.weight * 15000)) * 100)
                : 0;
          return (
            <StepRow
              key={step.name}
              icon={STATUS_ICON[st.status] || "⏳"}
              name={step.name}
              duration={st.status !== "waiting" ? st.durationMs : null}
              pct={pct}
              color={st.status === "completed" ? "#10a37f" : st.status === "running" ? "#2563EB" : "#9ca3af"}
            />
          );
        })}
      </div>
    </div>
  );
}

function StepRow({
  icon,
  name,
  duration,
  pct,
  color,
}: {
  icon: string;
  name: string;
  duration: number | null;
  pct: number;
  color: string;
}) {
  const barOuter: CSSProperties = {
    width: "100%",
    height: 6,
    background: "#e5e7eb",
    borderRadius: 3,
    overflow: "hidden",
    marginTop: 2,
  };
  const barInner: CSSProperties = {
    width: `${pct}%`,
    height: "100%",
    background: color,
    borderRadius: 3,
    transition: "width 0.3s ease",
  };
  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12 }}>
        <span>
          {icon} {name}
        </span>
        <span style={{ color: "#6b7280" }}>
          {duration !== null ? `${(duration / 1000).toFixed(1)}s` : ""}
        </span>
      </div>
      <div style={barOuter}>
        <div style={barInner} />
      </div>
    </div>
  );
}
