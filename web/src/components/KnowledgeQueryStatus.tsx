import styles from "../App.module.css";

interface KnowledgeQueryStatusProps {
  visible: boolean;
}

export function KnowledgeQueryStatus({ visible }: KnowledgeQueryStatusProps) {
  if (!visible) return null;
  return (
    <div style={{ padding: "12px 0", display: "flex", alignItems: "center", gap: 8 }}>
      <span className={styles.spinner} />
      <span style={{ fontSize: 14, color: "#555" }}>正在检索知识库...</span>
      <span style={{ fontSize: 12, color: "#999" }}>预计等待 2-5 秒</span>
    </div>
  );
}
