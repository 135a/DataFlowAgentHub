import { useMemo } from "react";

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
    return <p style={{ color: "#666", margin: "8px 0 0", fontSize: 14 }}>查询返回 0 行</p>;
  }

  return (
    <div style={{ overflowX: "auto", marginTop: 8 }}>
      <table
        style={{
          borderCollapse: "collapse",
          fontSize: 13,
          width: "100%",
          background: "#fff",
        }}
      >
        <thead>
          <tr>
            {columns.map((c) => (
              <th
                key={c}
                style={{
                  border: "1px solid #ccc",
                  padding: "8px 10px",
                  background: "#f0f4f8",
                  textAlign: "left",
                  whiteSpace: "nowrap",
                }}
              >
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i} style={{ background: i % 2 ? "#fafafa" : "#fff" }}>
              {columns.map((c) => (
                <td
                  key={c}
                  style={{
                    border: "1px solid #e0e0e0",
                    padding: "6px 10px",
                    maxWidth: 360,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                  }}
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
