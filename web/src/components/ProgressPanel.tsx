import { type CSSProperties } from "react";
import styles from "./ProgressPanel.module.css";

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
  return (
    <div className={styles.container}>
      <div className={styles.headerRow}>
        <strong>处理进度</strong>
        <span>
          已用 {fmt(elapsedMs)}
          {estimatedRemainingMs !== null && (
            <> · 预计剩余 {fmtRange(estimatedRemainingMs)}</>
          )}
        </span>
      </div>
      <div className={styles.stepsContainer}>
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
  const barInner: CSSProperties = {
    width: `${pct}%`,
    height: "100%",
    background: color,
    borderRadius: 3,
    transition: "width 0.3s ease",
  };
  return (
    <div>
      <div className={styles.stepRow}>
        <span className={styles.stepName}>
          {icon} {name}
        </span>
        <span className={styles.stepDuration}>
          {duration !== null ? `${(duration / 1000).toFixed(1)}s` : ""}
        </span>
      </div>
      <div className={styles.barOuter}>
        <div style={barInner} />
      </div>
    </div>
  );
}
