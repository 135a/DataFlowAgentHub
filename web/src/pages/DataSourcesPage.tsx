import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { apiJson } from "../api";
import { useIsAdmin, useIsOperator } from "../hooks/useRole";
import type { DataSource, DataSourcesResponse } from "../types/api";

interface EditFormData {
  name: string;
  kind: string;
  host: string;
  port: number;
  username: string;
  password?: string;
  database: string;
  sslmode?: string;
}

interface TestResponse {
  ok: boolean;
  error?: string;
}

export function DataSourcesPage() {
  const token = useMemo(() => localStorage.getItem("token"), []);
  const isAdmin = useIsAdmin();
  const isOperator = useIsOperator();
  const [items, setItems] = useState<DataSource[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ name: "", kind: "postgres", host: "", port: 5432, database: "", username: "", password: "", sslmode: "disable" });
  const [status, setStatus] = useState("");

  // edit state
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState({ name: "", kind: "postgres", host: "", port: 5432, database: "", username: "", password: "", sslmode: "disable" });

  // delete confirmation
  const [deleteId, setDeleteId] = useState<string | null>(null);

  // test state
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState("");

  const load = async () => {
    try {
      const j = await apiJson<DataSourcesResponse>("/v1/data-sources", { token });
      setItems(j.items || []);
    } catch {
      // load failure — keep existing items
    }
  };

  useEffect(() => { void load(); }, [token]);

  const submit = async () => {
    setStatus("creating...");
    try {
      await apiJson("/v1/data-sources", {
        method: "POST",
        token,
        body: JSON.stringify(form),
      });
      setStatus("创建成功");
      setShowForm(false);
      void load();
    } catch {
      setStatus("创建失败，请检查参数");
    }
  };

  const startEdit = (it: DataSource) => {
    setEditingId(it.id);
    setEditForm({
      name: it.name,
      kind: it.kind,
      host: it.host,
      port: it.port,
      database: it.database,
      username: "",
      password: "",
      sslmode: "disable",
    });
  };

  const doEdit = async () => {
    if (!editingId) return;
    setStatus("更新中...");
    try {
      const body: EditFormData = { ...editForm };
      if (!body.password) delete body.password; // 空密码表示不修改
      await apiJson(`/v1/data-sources/${editingId}`, {
        method: "PUT",
        token,
        body: JSON.stringify(body),
      });
      setStatus("更新成功");
      setEditingId(null);
      void load();
    } catch {
      setStatus("更新失败");
    }
  };

  const doDelete = async (id: string) => {
    try {
      await apiJson(`/v1/data-sources/${id}`, { method: "DELETE", token });
      setDeleteId(null);
      void load();
    } catch {
      setStatus("删除失败");
    }
  };

  const doTest = async (id: string) => {
    setTestingId(id);
    setTestResult("");
    try {
      const j = await apiJson<TestResponse>(`/v1/data-sources/${id}/test`, { method: "POST", token });
      setTestResult(j.ok ? "连接成功" : `连接失败: ${j.error || "unknown"}`);
    } catch {
      setTestResult("测试失败");
    }
    setTestingId(null);
  };

  return (
    <div style={{ fontFamily: "system-ui", padding: 16, maxWidth: 960, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>数据源管理</h1>
        <nav style={{ display: "flex", gap: 16 }}>
          <Link to="/tables">数据库表</Link>
          <Link to="/">返回</Link>
        </nav>
      </header>

      <button onClick={() => setShowForm(!showForm)} style={{ margin: "12px 0" }}>
        {showForm ? "取消" : "新建数据源"}
      </button>

      {showForm && (
        <div style={{ padding: 16, background: "#f5f5f5", borderRadius: 8, marginBottom: 16 }}>
          {(["name", "host", "database", "username", "password"] as const).map((f) => (
            <div key={f} style={{ marginBottom: 8 }}>
              <label style={{ width: 100, display: "inline-block" }}>{f}:</label>
              <input
                type={f === "password" ? "password" : "text"}
                value={String(form[f])}
                onChange={(e) => setForm({ ...form, [f]: e.target.value })}
              />
            </div>
          ))}
          <div style={{ marginBottom: 8 }}>
            <label style={{ width: 100, display: "inline-block" }}>port:</label>
            <input type="number" value={form.port} onChange={(e) => setForm({ ...form, port: Number(e.target.value) })} />
          </div>
          <button onClick={submit}>创建</button>
          {status && <span style={{ marginLeft: 12, fontSize: 13 }}>{status}</span>}
        </div>
      )}

      {testResult && (
        <p style={{ fontSize: 13, color: testResult.includes("成功") ? "#10a37f" : "#dc2626", margin: "8px 0" }}>
          {testResult}
        </p>
      )}

      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ textAlign: "left", borderBottom: "1px solid #ddd" }}>
            <th>名称</th><th>类型</th><th>主机</th><th>端口</th><th>数据库</th><th>状态</th>
            {(isAdmin || isOperator) && <th>操作</th>}
          </tr>
        </thead>
        <tbody>
          {items.map((it) => (
            <tr key={it.id} style={{ borderBottom: "1px solid #eee" }}>
              <td>{it.name}</td>
              <td>{it.kind}</td>
              <td>{it.host}</td>
              <td>{it.port}</td>
              <td>{it.database}</td>
              <td>{it.has_password ? "connected" : "no password"}</td>
              {(isAdmin || isOperator) && (
                <td>
                  <div style={{ display: "flex", gap: 4 }}>
                    {isOperator && (
                      <button
                        onClick={() => doTest(it.id)}
                        disabled={testingId === it.id}
                        style={{ fontSize: 11, padding: "2px 6px" }}
                      >
                        {testingId === it.id ? "测试中..." : "测试"}
                      </button>
                    )}
                    {isAdmin && (
                      <>
                        <button onClick={() => startEdit(it)} style={{ fontSize: 11, padding: "2px 6px" }}>编辑</button>
                        {deleteId === it.id ? (
                          <span style={{ fontSize: 11 }}>
                            <button onClick={() => doDelete(it.id)} style={{ color: "red", padding: "2px 6px" }}>确认</button>
                            <button onClick={() => setDeleteId(null)} style={{ padding: "2px 6px" }}>取消</button>
                          </span>
                        ) : (
                          <button onClick={() => setDeleteId(it.id)} style={{ fontSize: 11, color: "#dc2626", padding: "2px 6px" }}>删除</button>
                        )}
                      </>
                    )}
                  </div>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>

      {/* edit modal */}
      {editingId && (
        <div style={{
          position: "fixed", inset: 0, background: "rgba(0,0,0,0.4)",
          display: "flex", alignItems: "center", justifyContent: "center", zIndex: 100,
        }}>
          <div style={{ background: "#fff", padding: 20, borderRadius: 8, minWidth: 380 }}>
            <h3 style={{ margin: "0 0 12px" }}>编辑数据源</h3>
            {(["name", "host", "database", "username", "password"] as const).map((f) => (
              <div key={f} style={{ marginBottom: 8 }}>
                <label style={{ width: 100, display: "inline-block" }}>{f}:</label>
                <input
                  type={f === "password" ? "password" : "text"}
                  placeholder={f === "password" ? "留空则不修改" : ""}
                  value={String(editForm[f])}
                  onChange={(e) => setEditForm({ ...editForm, [f]: e.target.value })}
                />
              </div>
            ))}
            <div style={{ marginBottom: 8 }}>
              <label style={{ width: 100, display: "inline-block" }}>port:</label>
              <input type="number" value={editForm.port} onChange={(e) => setEditForm({ ...editForm, port: Number(e.target.value) })} />
            </div>
            <div style={{ display: "flex", gap: 8, marginTop: 12 }}>
              <button onClick={doEdit}>保存</button>
              <button onClick={() => setEditingId(null)}>取消</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
