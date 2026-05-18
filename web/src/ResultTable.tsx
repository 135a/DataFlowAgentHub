import { useMemo } from "react";
import styles from "./ResultTable.module.css";

function formatCell(v: unknown): string {
  if (v === null || v === undefined) return "";
  if (typeof v === "object") return JSON.stringify(v);
  return String(v);
}

/** 从查询结果行渲染表格；列顺序为首次出现顺序（跨行并集）。 */
export function ResultTable({ rows }: { rows: Record<string, unknown>[] }) {
  const columns = useMemo(() => {
    const order: string[] = [];
    const seen = new Set<string>();
    for (const r of rows) {
      for (const k of Object.keys(r)) {
        if (!seen.has(k)) {
          seen.add(k);
          order.push(k);
        }
      }
    }
    return order;
  }, [rows]);

  if (!rows.length) {
    return <p className={styles.emptyRow}>查询返回 0 行</p>;
  }

  return (
    <div className={styles.wrapper}>
      <table className={styles.table}>
        <thead>
          <tr>
            {columns.map((c) => (
              <th key={c} className={styles.headerCell}>
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i} className={i % 2 ? styles.oddRow : styles.evenRow}>
              {columns.map((c) => (
                <td
                  key={c}
                  className={styles.dataCell}
                  title={formatCell(r[c])}
                >
                  {formatCell(r[c])}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
