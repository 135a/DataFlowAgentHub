import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { apiFetch } from "../api";

export function RegisterPage() {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");
  const [ok, setOk] = useState("");

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    setOk("");
    const r = await apiFetch("/v1/auth/self-register", {
      method: "POST",
      body: JSON.stringify({ name, phone, password }),
    });
    if (!r.ok) {
      const text = await r.text().catch(() => "注册失败");
      setErr(text || "注册失败");
      return;
    }
    setOk("注册成功，请登录");
    setTimeout(() => navigate("/login"), 1500);
  }

  return (
    <div style={{ maxWidth: 360, margin: "80px auto", fontFamily: "system-ui" }}>
      <h1>注册</h1>
      <form onSubmit={onSubmit}>
        <div>
          <label>姓名</label>
          <input value={name} onChange={(e) => setName(e.target.value)} style={{ width: "100%" }} required />
        </div>
        <div style={{ marginTop: 8 }}>
          <label>手机号</label>
          <input value={phone} onChange={(e) => setPhone(e.target.value)} style={{ width: "100%" }} required />
        </div>
        <div style={{ marginTop: 8 }}>
          <label>密码</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} style={{ width: "100%" }} required />
        </div>
        {err && <p style={{ color: "crimson" }}>{err}</p>}
        {ok && <p style={{ color: "#10a37f" }}>{ok}</p>}
        <button type="submit" style={{ marginTop: 12 }}>注册</button>
      </form>
      <p style={{ marginTop: 12, fontSize: 14 }}>
        <Link to="/login">已有账号？登录</Link>
      </p>
    </div>
  );
}
