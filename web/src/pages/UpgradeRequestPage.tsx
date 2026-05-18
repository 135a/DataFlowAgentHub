import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../api";

export function UpgradeRequestPage() {
  const token = localStorage.getItem("token") || "";
  const [requestedRole, setRequestedRole] = useState("data_admin");
  const [reason, setReason] = useState("");
  const [err, setErr] = useState("");
  const [ok, setOk] = useState("");

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr("");
    setOk("");
    const r = await apiFetch("/v1/auth/upgrade-request", {
      method: "POST",
      body: JSON.stringify({ requested_role: requestedRole, reason }),
    });
    if (!r.ok) {
      const text = await r.text().catch(() => "提交失败");
      setErr(text || "提交失败");
      return;
    }
    setOk("申请已提交，等待管理员审核");
  }

  return (
    <div style={{ maxWidth: 480, margin: "80px auto", fontFamily: "system-ui" }}>
      <h1>权限升级申请</h1>
      <form onSubmit={onSubmit}>
        <div>
          <label>目标角色</label>
          <select
            value={requestedRole}
            onChange={(e) => setRequestedRole(e.target.value)}
            style={{ width: "100%", marginTop: 4 }}
          >
            <option value="data_admin">数据管理员 (data_admin)</option>
            <option value="read_only_visitor">只读访客 (read_only_visitor)</option>
          </select>
        </div>
        <div style={{ marginTop: 12 }}>
          <label>申请原因</label>
          <textarea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={4}
            style={{ width: "100%", marginTop: 4, padding: "4px 8px" }}
            required
          />
        </div>
        {err && <p style={{ color: "crimson" }}>{err}</p>}
        {ok && <p style={{ color: "#10a37f" }}>{ok}</p>}
        <button type="submit" style={{ marginTop: 12 }}>提交申请</button>
      </form>
      <p style={{ marginTop: 12, fontSize: 14 }}>
        <Link to="/">返回聊天</Link>
      </p>
    </div>
  );
}
