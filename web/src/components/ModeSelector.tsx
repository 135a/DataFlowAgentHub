import styles from "./ModeSelector.module.css";

interface ModeSelectorProps {
  mode: "quick" | "deep";
  onChange: (mode: "quick" | "deep") => void;
}

function ModeButton({
  selected,
  isQuick,
  label,
  sub,
  onClick,
}: {
  selected: boolean;
  isQuick: boolean;
  label: string;
  sub: string;
  onClick: () => void;
}) {
  const borderClass = isQuick ? styles.buttonQuick : styles.buttonDeep;
  const activeClass = selected
    ? (isQuick ? styles.buttonActiveQuick : styles.buttonActiveDeep)
    : (isQuick ? styles.buttonInactiveQuick : styles.buttonInactiveDeep);
  const stateClass = selected ? styles.buttonActive : styles.buttonInactive;

  return (
    <button
      type="button"
      className={`${styles.button} ${borderClass} ${stateClass} ${activeClass}`}
      onClick={onClick}
    >
      <span>{label}</span>
      <span className={`${styles.subText} ${selected ? styles.subTextActive : ""}`}>
        {sub}
      </span>
    </button>
  );
}

export function ModeSelector({ mode, onChange }: ModeSelectorProps) {
  return (
    <div className={styles.container}>
      <ModeButton
        selected={mode === "quick"}
        isQuick={true}
        label="⚡ 快速查询"
        sub="秒出 SQL 结果"
        onClick={() => onChange("quick")}
      />
      <ModeButton
        selected={mode === "deep"}
        isQuick={false}
        label="🔬 深度分析"
        sub="图表 + 报告 + 分析"
        onClick={() => onChange("deep")}
      />
    </div>
  );
}
