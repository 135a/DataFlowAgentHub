import styles from "./QuerySourceSelector.module.css";

type QuerySource = "knowledge" | "dataset";

interface QuerySourceSelectorProps {
  value: QuerySource;
  onChange: (source: QuerySource) => void;
}

function SourceButton({
  selected,
  isKnowledge,
  label,
  sub,
  onClick,
}: {
  selected: boolean;
  isKnowledge: boolean;
  label: string;
  sub: string;
  onClick: () => void;
}) {
  const borderClass = isKnowledge ? styles.buttonKnowledge : styles.buttonDataset;
  const activeClass = selected
    ? (isKnowledge ? styles.buttonActiveKnowledge : styles.buttonActiveDataset)
    : (isKnowledge ? styles.buttonInactiveKnowledge : styles.buttonInactiveDataset);
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

export type { QuerySource };
export { QuerySourceSelector };

function QuerySourceSelector({ value, onChange }: QuerySourceSelectorProps) {
  return (
    <div className={styles.container}>
      <SourceButton
        selected={value === "knowledge"}
        isKnowledge={true}
        label="📄 知识库查询"
        sub="RAG + LLM 问答"
        onClick={() => onChange("knowledge")}
      />
      <SourceButton
        selected={value === "dataset"}
        isKnowledge={false}
        label="🗃️ 数据集查询"
        sub="NL2SQL 数据分析"
        onClick={() => onChange("dataset")}
      />
    </div>
  );
}
