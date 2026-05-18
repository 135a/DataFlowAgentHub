import { useCallback, useEffect, useState } from "react";
import { Link, useParams, useNavigate } from "react-router-dom";
import { apiJson } from "../api";
import { useIsDataAdmin } from "../hooks/useRole";
import type { DatasetTable, DatasetTablesResponse, TableField, TableDetailResponse, CreateTableResponseV2 } from "../types/api";

const FIELD_TYPES = ["VARCHAR", "INT", "BIGINT", "DECIMAL", "DATE", "DATETIME", "TEXT", "BOOLEAN", "FLOAT", "DOUBLE"];

interface FieldDef {
  name: string;
  field_type: string;
  field_length: number;
  is_nullable: boolean;
}

export function DatasetTablesPage() {
  const { did } = useParams<{ did: string }>();
  const navigate = useNavigate();
  const token = localStorage.getItem("token") || "";
  const isDataAdmin = useIsDataAdmin();

  const [tables, setTables] = useState<DatasetTable[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");

  // 创建表
  const [showCreate, setShowCreate] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createDisplayName, setCreateDisplayName] = useState("");
  const [fields, setFields] = useState<FieldDef[]>([{ name: "", field_type: "VARCHAR", field_length: 255, is_nullable: true }]);

  // 表详情
  const [detailId, setDetailId] = useState<string | null>(null);
  const [detailFields, setDetailFields] = useState<TableField[]>([]);

  // 删除确认
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const loadTables = useCallback(async () => {
    if (!did) return;
    setLoading(true);
    try {
      const j = await apiJson<DatasetTablesResponse>(`/v1/datasets/${did}/tables`, { token });
      setTables(j.tables || []);
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [did, token]);

  useEffect(() => { void loadTables(); }, [loadTables]);

  function addField() {
    setFields([...fields, { name: "", field_type: "VARCHAR", field_length: 255, is_nullable: true }]);
  }

  function removeField(idx: number) {
    setFields(fields.filter((_, i) => i !== idx));
  }

  function updateField(idx: number, key: keyof FieldDef, value: string | number | boolean) {
    const next = [...fields];
    (next[idx] as any)[key] = value;
    setFields(next);
  }

  async function handleCreate() {
    if (!createName.trim() || !did) return;
    const validFields = fields.filter(f => f.name.trim());
    if (validFields.length === 0) { setErr("至少需要一个字段"); return; }
    try {
      await apiJson<CreateTableResponseV2>(`/v1/datasets/${did}/tables`, {
        method: "POST", token,
        body: JSON.stringify({
          name: createName.trim(),
          display_name: createDisplayName.trim() || undefined,
          fields: validFields,
        }),
      });
      setShowCreate(false);
      setCreateName("");
      setCreateDisplayName("");
      setFields([{ name: "", field_type: "VARCHAR", field_length: 255, is_nullable: true }]);
      void loadTables();
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "创建失败");
    }
  }

  async function loadDetail(tableId: string) {
    if (!did) return;
    if (detailId === tableId) {
      setDetailId(null);
      return;
    }
    try {
      const j = await apiJson<TableDetailResponse>(`/v1/datasets/${did}/tables/${tableId}`, { token });
      setDetailFields(j.fields || []);
      setDetailId(tableId);
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "加载详情失败");
    }
  }

  async function handleDelete(tableId: string) {
    if (!did) return;
    try {
      await apiJson(`/v1/datasets/${did}/tables/${tableId}`, { method: "DELETE", token });
      setDeleteId(null);
      void loadTables();
      if (detailId === tableId) setDetailId(null);
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "删除失败");
    }
  }

  if (loading) return <div style={{ padding: 24 }}>加载中...</div>;

  return (
    <div style={{ maxWidth: 900, margin: "0 auto", padding: 24, fontFamily: "system-ui" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <div>
          <Link to="/datasets" style={{ fontSize: 14 }}>← 返回数据集</Link>
          <h1 style={{ margin: "8px 0" }}>数据表管理</h1>
        </div>
        {isDataAdmin && (
          <button onClick={() => setShowCreate(!showCreate)}>
            {showCreate ? "取消" : "创建表"}
          </button>
        )}
      </div>

      {err && <p style={{ color: "crimson" }}>{err}</p>}

      {showCreate && isDataAdmin && (
        <div style={{ padding: 12, border: "1px solid #ddd", borderRadius: 8, marginBottom: 16, background: "#fafafa" }}>
          <h3>创建数据表</h3>
          <div style={{ marginBottom: 8 }}>
            <input
              placeholder="表名（必填）"
              value={createName}
              onChange={(e) => setCreateName(e.target.value)}
              style={{ width: "100%", padding: "4px 8px", marginBottom: 4 }}
            />
            <input
              placeholder="显示名称（可选）"
              value={createDisplayName}
              onChange={(e) => setCreateDisplayName(e.target.value)}
              style={{ width: "100%", padding: "4px 8px" }}
            />
          </div>
          <h4 style={{ margin: "8px 0" }}>字段定义</h4>
          {fields.map((f, idx) => (
            <div key={idx} style={{ display: "flex", gap: 4, marginBottom: 4, alignItems: "center" }}>
              <input
                placeholder="字段名"
                value={f.name}
                onChange={(e) => updateField(idx, "name", e.target.value)}
                style={{ width: 120, padding: "2px 4px", fontSize: 12 }}
              />
              <select
                value={f.field_type}
                onChange={(e) => updateField(idx, "field_type", e.target.value)}
                style={{ fontSize: 12 }}
              >
                {FIELD_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
              {(f.field_type === "VARCHAR" || f.field_type === "DECIMAL") && (
                <input
                  type="number"
                  placeholder="长度"
                  value={f.field_length || ""}
                  onChange={(e) => updateField(idx, "field_length", parseInt(e.target.value) || 0)}
                  style={{ width: 60, padding: "2px 4px", fontSize: 12 }}
                />
              )}
              <label style={{ fontSize: 12 }}>
                <input
                  type="checkbox"
                  checked={f.is_nullable}
                  onChange={(e) => updateField(idx, "is_nullable", e.target.checked)}
                /> 可空
              </label>
              {fields.length > 1 && (
                <button onClick={() => removeField(idx)} style={{ fontSize: 12, color: "crimson" }}>删除</button>
              )}
            </div>
          ))}
          <button onClick={addField} style={{ fontSize: 12, marginTop: 4 }}>+ 添加字段</button>
          <div style={{ marginTop: 8 }}>
            <button onClick={handleCreate}>确认创建</button>
          </div>
        </div>
      )}

      {tables.length === 0 && !loading && (
        <p style={{ color: "#888" }}>暂无数据表</p>
      )}

      {tables.map((t) => (
        <div key={t.id} style={{ border: "1px solid #e0e0e0", borderRadius: 8, padding: 12, marginBottom: 8, background: "#fafafa" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <div>
              <strong
                style={{ cursor: "pointer", fontSize: 15 }}
                onClick={() => loadDetail(t.id)}
              >
                {t.display_name || t.name}
              </strong>
              <span style={{ fontSize: 12, color: "#888", marginLeft: 8 }}>
                MySQL: {t.mysql_table_name} · 状态: {t.status}
              </span>
            </div>
            {isDataAdmin && (
              <div style={{ display: "flex", gap: 8 }}>
                <Link
                  to={`/datasets/${did}/sql-terminal?table=${t.mysql_table_name}`}
                  style={{ fontSize: 12, color: "#1976d2", textDecoration: "none" }}
                >
                  SQL 终端
                </Link>
                <button onClick={() => setDeleteId(t.id)} style={{ fontSize: 12, color: "crimson" }}>删除</button>
              </div>
            )}
          </div>

          {detailId === t.id && (
            <div style={{ marginTop: 8, padding: 8, background: "#fff", borderRadius: 6 }}>
              <h4 style={{ margin: "0 0 4px", fontSize: 13 }}>字段列表</h4>
              {detailFields.length === 0 && <p style={{ fontSize: 12, color: "#888" }}>加载中...</p>}
              <table style={{ width: "100%", fontSize: 12, borderCollapse: "collapse" }}>
                <thead>
                  <tr style={{ background: "#eee" }}>
                    <th style={{ padding: 4, textAlign: "left" }}>#</th>
                    <th style={{ padding: 4, textAlign: "left" }}>字段名</th>
                    <th style={{ padding: 4, textAlign: "left" }}>类型</th>
                    <th style={{ padding: 4, textAlign: "left" }}>长度</th>
                    <th style={{ padding: 4, textAlign: "left" }}>可空</th>
                  </tr>
                </thead>
                <tbody>
                  {detailFields.map((f) => (
                    <tr key={f.id} style={{ borderTop: "1px solid #eee" }}>
                      <td style={{ padding: 4 }}>{f.ordinal_position}</td>
                      <td style={{ padding: 4 }}>{f.display_name || f.name}</td>
                      <td style={{ padding: 4 }}>{f.field_type}</td>
                      <td style={{ padding: 4 }}>{f.field_length > 0 ? f.field_length : "-"}</td>
                      <td style={{ padding: 4 }}>{f.is_nullable ? "是" : "否"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {deleteId === t.id && (
            <div style={{ marginTop: 8, padding: 8, background: "#fff0f0", borderRadius: 6 }}>
              <p style={{ fontSize: 13, margin: "0 0 4px" }}>确认删除表 "{t.name}"？</p>
              <div style={{ display: "flex", gap: 4 }}>
                <button onClick={() => handleDelete(t.id)} style={{ color: "crimson" }}>确认删除</button>
                <button onClick={() => setDeleteId(null)}>取消</button>
              </div>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
