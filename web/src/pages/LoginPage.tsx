import { useState, type FormEvent } from "react";
import { apiFetch } from "../api";

export function LoginPage() {
  const [email, setEmail] = useState("admin@demo.local");
  const [password, setPassword] = useState("changeme");
  const [err, setErr] = useState("");

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    const r = await apiFetch("/v1/auth/login", { method: "POST", body: JSON.stringify({ email, password }) });
    if (!r.ok) {
      setErr("登录失败");
      return;
    }
    const j = await r.json();
    localStorage.setItem("token", j.access_token as string);
    window.location.href = "/";
  }

  return (
    <div style={{ maxWidth: 360, margin: "80px auto", fontFamily: "system-ui" }}>
      <h1>登录</h1>
      <form onSubmit={onSubmit}>
        <div>
          <label>Email</label>
          <input value={email} onChange={(e) => setEmail(e.target.value)} style={{ width: "100%" }} />
        </div>
        <div style={{ marginTop: 8 }}>
          <label>密码</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} style={{ width: "100%" }} />
        </div>
        {err && <p style={{ color: "crimson" }}>{err}</p>}
        <button type="submit" style={{ marginTop: 12 }}>进入</button>
      </form>
      <p style={{ fontSize: 12, color: "#666" }}>开发环境默认走 Vite 代理到后端；生产设置 VITE_API_BASE_URL。</p>
    </div>
  );
}
