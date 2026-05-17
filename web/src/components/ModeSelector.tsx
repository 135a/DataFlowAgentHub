import type { CSSProperties } from "react";

interface ModeSelectorProps {
  mode: "quick" | "deep";
  onChange: (mode: "quick" | "deep") => void;
}

const QUICK = "#2563EB";
const DEEP = "#7C3AED";

function ModeButton({
  selected,
  color,
  label,
  sub,
  onClick,
}: {
  selected: boolean;
  color: string;
  label: string;
  sub: string;
  onClick: () => void;
}) {
  const style: CSSProperties = {
    padding: "6px 18px",
    fontSize: 14,
    fontWeight: 500,
    borderRadius: 20,
    cursor: "pointer",
    border: `1.5px solid ${color}`,
    background: selected ? color : "transparent",
    color: selected ? "#fff" : color,
    transition: "all 0.15s ease",
    display: "flex",
    flexDirection: "column",
    alignItems: "center",
    lineHeight: 1.4,
    outline: "none",
  };
  return (
    <button type="button" style={style} onClick={onClick}>
      <span>{label}</span>
      <span style={{ fontSize: 10, opacity: selected ? 0.9 : 0.65 }}>{sub}</span>
    </button>
  );
}

export function ModeSelector({ mode, onChange }: ModeSelectorProps) {
  const container: CSSProperties = {
    display: "flex",
    gap: 10,
    marginBottom: 8,
    flexWrap: "wrap",
  };
  return (
    <div style={container}>
      <ModeButton
        selected={mode === "quick"}
        color={QUICK}
        label="⚡ 快速查询"
        sub="秒出 SQL 结果"
        onClick={() => onChange("quick")}
      />
      <ModeButton
        selected={mode === "deep"}
        color={DEEP}
        label="🔬 深度分析"
        sub="图表 + 报告 + 分析"
        onClick={() => onChange("deep")}
      />
    </div>
  );
}
