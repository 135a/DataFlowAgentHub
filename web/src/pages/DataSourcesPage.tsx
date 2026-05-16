import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../api";

export function DataSourcesPage() {
  const token = useMemo(() => localStorage.getItem("token"), []);
  const [items, setItems] = useState<any[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ name: "", kind: "postgres", host: "", port: 5432, database: "", username: "", password: "", sslmode: "disable" });
  const [status, setStatus] = useState("");

  const load = async () => {
    const r = await apiFetch("/v1/data-sources", { token });
    if (r.ok) {
      const j = await r.json();
      setItems(j.items || []);
    }
  };

  useEffect(() => { void load(); }, [token]);

  const submit = async () => {
    setStatus("creating...");
    const r = await apiFetch("/v1/data-sources", {
      method: "POST",
      token,
      body: JSON.stringify(form),
    });
    if (r.ok) {
      setStatus("created");
      setShowForm(false);
      void load();
    } else {
      const j = await r.json().catch(() => ({ error: "unknown" }));
      setStatus(`error: ${(j as any).error || r.status}`);
    }
  };

  return (
    <div style={{ fontFamily: "system-ui", padding: 16, maxWidth: 960, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>数据源管理</h1>
        <Link to="/">返回</Link>
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

      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ textAlign: "left", borderBottom: "1px solid #ddd" }}>
            <th>名称</th><th>类型</th><th>主机</th><th>端口</th><th>数据库</th><th>状态</th>
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
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
