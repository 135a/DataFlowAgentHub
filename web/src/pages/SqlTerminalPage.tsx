import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useParams, useSearchParams, Link } from "react-router-dom";
import { apiJson } from "../api";
import type { Dataset, DatasetsResponse, DatasetTable, DatasetTablesResponse, SqlExecuteResponse } from "../types/api";

export function SqlTerminalPage() {
  const token = useMemo(() => localStorage.getItem("token"), []);
  const { did } = useParams<{ did: string }>();
  const [searchParams] = useSearchParams();
  const tableName = searchParams.get("table");

  const [datasets, setDatasets] = useState<Dataset[]>([]);
  const [selectedDatasetId, setSelectedDatasetId] = useState(did || "");
  const [tables, setTables] = useState<DatasetTable[]>([]);
  const [sql, setSql] = useState("");
  const [executing, setExecuting] = useState(false);
  const [result, setResult] = useState<SqlExecuteResponse | null>(null);
  const [error, setError] = useState("");

  // 加载数据集列表
  useEffect(() => {
    if (!token) return;
    apiJson<DatasetsResponse>("/v1/datasets", { token })
      .then(j => setDatasets(j.datasets || []))
      .catch(() => {});
  }, [token]);

  // 如果 URL 中指定了 did，加载该数据集下的表
  useEffect(() => {
    if (!token || !selectedDatasetId) return;
    apiJson<DatasetTablesResponse>(`/v1/datasets/${selectedDatasetId}/tables`, { token })
      .then(j => setTables(j.tables || []))
      .catch(() => {});
  }, [token, selectedDatasetId]);

  // 快速浏览模式：从表管理页跳转时自动填入 SQL
  useEffect(() => {
    if (tableName && selectedDatasetId) {
      setSql(`SELECT * FROM \`${tableName}\` LIMIT 50`);
    }
  }, [tableName, selectedDatasetId]);

  // 数据集切换
  const handleDatasetChange = (datasetID: string) => {
    setSelectedDatasetId(datasetID);
    setResult(null);
    setError("");
  };

  // 自动补全表名到 SQL 输入框
  const insertTableName = (name: string) => {
    setSql(prev => prev + `\`${name}\``);
  };

  const execute = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!selectedDatasetId || !sql.trim()) return;

    setExecuting(true);
    setError("");
    setResult(null);

    try {
      const j = await apiJson<SqlExecuteResponse>("/v1/data/execute", {
        method: "POST",
        token,
        body: JSON.stringify({ dataset_id: selectedDatasetId, sql: sql.trim() }),
      });
      setResult(j);
    } catch (err: any) {
      setError(err.message || "执行失败");
    } finally {
      setExecuting(false);
    }
  };

  const renderResult = () => {
    if (!result) return null;

    if (!result.ok) {
      return <div style={{ color: "#d32f2f", padding: 12, background: "#fef2f2", borderRadius: 6 }}>执行失败</div>;
    }

    if (result.type === "select") {
      const cols = result.columns || [];
      const rows = result.rows || [];

      return (
        <div>
          <div style={{ marginBottom: 8, fontSize: 13, color: "#666" }}>
            {result.truncated
              ? `仅显示前 ${rows.length} 行，共 ${result.total_count} 行`
              : `共 ${result.total_count} 行`}
          </div>
          <div style={{ overflowX: "auto" }}>
            <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
              <thead>
                <tr style={{ background: "#f5f5f5", textAlign: "left" }}>
                  {cols.map((col, i) => (
                    <th key={i} style={{ padding: "8px 12px", borderBottom: "2px solid #ddd", whiteSpace: "nowrap", fontWeight: 600 }}>
                      {col}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map((row, ri) => (
                  <tr key={ri} style={{ borderBottom: "1px solid #eee" }}>
                    {cols.map((col, ci) => (
                      <td key={ci} style={{ padding: "6px 12px", maxWidth: 300, overflow: "hidden", textOverflow: "ellipsis" }}>
                        {String(row[col] ?? "")}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      );
    }

    // INSERT / UPDATE
    return (
      <div style={{ padding: 12, background: "#f0fdf4", borderRadius: 6, color: "#166534" }}>
        {result.type === "insert" ? "插入" : "更新"}成功，影响行数: <strong>{result.rows_affected ?? 0}</strong>
      </div>
    );
  };

  return (
    <div style={{ fontFamily: "system-ui", padding: 16, maxWidth: 960, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <h1 style={{ margin: 0 }}>SQL 终端</h1>
        <nav style={{ display: "flex", gap: 16 }}>
          <Link to="/datasets">数据集管理</Link>
          <Link to="/">返回</Link>
        </nav>
      </header>

      {/* 数据集选择器 */}
      <div style={{ marginBottom: 12, display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
        <div>
          <label style={{ marginRight: 8, fontWeight: 500 }}>数据集:</label>
          <select
            value={selectedDatasetId}
            onChange={e => handleDatasetChange(e.target.value)}
            style={{ padding: "6px 12px", borderRadius: 4, border: "1px solid #ccc", minWidth: 200 }}
          >
            <option value="">-- 选择数据集 --</option>
            {datasets.map(ds => (
              <option key={ds.id} value={ds.id}>{ds.name}</option>
            ))}
          </select>
        </div>
      </div>

      {/* 数据表快捷插入 */}
      {tables.length > 0 && selectedDatasetId && (
        <div style={{ marginBottom: 12, fontSize: 13 }}>
          <span style={{ fontWeight: 500, marginRight: 8 }}>表:</span>
          {tables.filter(t => t.status === "active").map(t => (
            <button
              key={t.id}
              type="button"
              onClick={() => insertTableName(t.mysql_table_name)}
              style={{
                margin: "2px 4px", padding: "2px 8px", fontSize: 12,
                border: "1px solid #ccc", borderRadius: 4, background: "#f9f9f9",
                cursor: "pointer",
              }}
              title="点击插入表名"
            >
              {t.mysql_table_name}
            </button>
          ))}
        </div>
      )}

      {/* SQL 输入与执行 */}
      <form onSubmit={execute}>
        <div style={{ marginBottom: 8 }}>
          <textarea
            value={sql}
            onChange={e => setSql(e.target.value)}
            rows={6}
            placeholder="输入 SQL 语句（SELECT / INSERT / UPDATE）..."
            style={{
              width: "100%", padding: 12, fontFamily: "'Courier New', monospace", fontSize: 14,
              border: "1px solid #ccc", borderRadius: 6, resize: "vertical", boxSizing: "border-box",
            }}
          />
        </div>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <button
            type="submit"
            disabled={executing || !selectedDatasetId || !sql.trim()}
            style={{
              padding: "8px 24px", background: executing ? "#999" : "#1976d2",
              color: "#fff", border: "none", borderRadius: 4, cursor: "pointer", fontWeight: 500,
            }}
          >
            {executing ? "执行中..." : "执行"}
          </button>
          <button
            type="button"
            onClick={() => { setSql(""); setResult(null); setError(""); }}
            style={{
              padding: "8px 16px", background: "#f5f5f5",
              color: "#333", border: "1px solid #ccc", borderRadius: 4, cursor: "pointer",
            }}
          >
            清空
          </button>
          {tableName && (
            <span style={{ fontSize: 12, color: "#888" }}>
              快速浏览模式: {tableName}
            </span>
          )}
        </div>
      </form>

      {/* 错误信息 */}
      {error && (
        <div style={{ marginTop: 16, padding: 12, background: "#fef2f2", borderRadius: 6, color: "#d32f2f", fontSize: 13, whiteSpace: "pre-wrap" }}>
          <strong>错误:</strong> {error}
        </div>
      )}

      {/* 结果展示 */}
      {result && (
        <div style={{ marginTop: 16 }}>
          {renderResult()}
        </div>
      )}
    </div>
  );
}
