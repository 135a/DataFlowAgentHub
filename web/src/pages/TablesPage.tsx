import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { apiJson } from "../api";

interface ColumnInfo {
  name: string;
  type: string;
  nullable: boolean;
}

interface TableInfo {
  name: string;
  columns: ColumnInfo[];
  row_estimate: number;
  last_vacuum?: string;
}

interface TablesResponse {
  tables: TableInfo[];
}

export function TablesPage() {
  const token = useMemo(() => localStorage.getItem("token"), []);
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [search, setSearch] = useState("");
  const [expanded, setExpanded] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    apiJson<TablesResponse>("/v1/schema/tables", { token })
      .then(j => setTables(j.tables || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [token]);

  const filtered = useMemo(() => {
    if (!search.trim()) return tables;
    const q = search.toLowerCase();
    return tables.filter(t => t.name.toLowerCase().includes(q));
  }, [tables, search]);

  return (
    <div style={{ fontFamily: "system-ui", padding: 16, maxWidth: 960, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>数据库表结构</h1>
        <nav style={{ display: "flex", gap: 16 }}>
          <Link to="/data-sources">数据源</Link>
          <Link to="/knowledge">知识库</Link>
          <Link to="/">返回</Link>
        </nav>
      </header>

      <div style={{ margin: "12px 0" }}>
        <input
          placeholder="搜索表名..."
          value={search}
          onChange={e => setSearch(e.target.value)}
          style={{ padding: "6px 10px", fontSize: 14, width: 260 }}
        />
      </div>

      {loading ? (
        <p>加载中...</p>
      ) : filtered.length === 0 ? (
        <p style={{ color: "#888" }}>{search ? "无匹配表" : "暂无用户表"}</p>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {filtered.map(t => {
            const isOpen = expanded === t.name;
            return (
              <div
                key={t.name}
                style={{
                  border: "1px solid #e0e0e0",
                  borderRadius: 8,
                  background: "#fff",
                  overflow: "hidden",
                }}
              >
                <button
                  type="button"
                  onClick={() => setExpanded(isOpen ? null : t.name)}
                  style={{
                    width: "100%",
                    textAlign: "left",
                    padding: "10px 14px",
                    border: "none",
                    background: "#f8f9fa",
                    cursor: "pointer",
                    fontSize: 15,
                    fontWeight: 600,
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                  }}
                >
                  <span>{t.name}</span>
                  <span style={{ fontSize: 12, color: "#888", fontWeight: 400 }}>
                    {t.row_estimate.toLocaleString()} 行
                    {t.last_vacuum ? ` · 更新于 ${new Date(t.last_vacuum).toLocaleDateString()}` : ""}
                    {" "}{isOpen ? "▲" : "▼"}
                  </span>
                </button>
                {isOpen && (
                  <div style={{ padding: "4px 0" }}>
                    <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
                      <thead>
                        <tr style={{ borderBottom: "1px solid #e0e0e0", color: "#666" }}>
                          <th style={{ textAlign: "left", padding: "6px 14px" }}>字段名</th>
                          <th style={{ textAlign: "left", padding: "6px 14px" }}>类型</th>
                          <th style={{ textAlign: "left", padding: "6px 14px" }}>可空</th>
                        </tr>
                      </thead>
                      <tbody>
                        {t.columns.map(c => (
                          <tr key={c.name} style={{ borderBottom: "1px solid #f0f0f0" }}>
                            <td style={{ padding: "5px 14px", fontFamily: "monospace" }}>{c.name}</td>
                            <td style={{ padding: "5px 14px", color: "#555" }}>{c.type}</td>
                            <td style={{ padding: "5px 14px" }}>{c.nullable ? "是" : "否"}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
