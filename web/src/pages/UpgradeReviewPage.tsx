import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { apiJson } from "../api";
import type { UpgradeRequest, UpgradeRequestsResponse } from "../types/api";

export function UpgradeReviewPage() {
  const token = localStorage.getItem("token") || "";
  const [requests, setRequests] = useState<UpgradeRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");
  const [reviewNotes, setReviewNotes] = useState("");

  const loadRequests = useCallback(async () => {
    setLoading(true);
    try {
      const j = await apiJson<UpgradeRequestsResponse>("/v1/auth/upgrade-requests?status=pending", { token });
      setRequests(j.requests || []);
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => { void loadRequests(); }, [loadRequests]);

  async function handleReview(id: string, action: "approve" | "reject") {
    try {
      await apiJson(`/v1/auth/upgrade-requests/${id}`, {
        method: "PUT", token,
        body: JSON.stringify({ action, review_notes: reviewNotes }),
      });
      setReviewNotes("");
      void loadRequests();
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "操作失败");
    }
  }

  return (
    <div style={{ maxWidth: 800, margin: "0 auto", padding: 24, fontFamily: "system-ui" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>权限升级审批</h1>
        <Link to="/" style={{ fontSize: 14 }}>返回聊天</Link>
      </div>

      {err && <p style={{ color: "crimson" }}>{err}</p>}

      {loading && <p>加载中...</p>}

      {!loading && requests.length === 0 && (
        <p style={{ color: "#888" }}>暂无待审批的申请</p>
      )}

      {requests.map((req) => (
        <div key={req.id} style={{ border: "1px solid #e0e0e0", borderRadius: 8, padding: 16, marginBottom: 12, background: "#fafafa" }}>
          <div style={{ marginBottom: 8 }}>
            <p style={{ margin: 0 }}><strong>{req.user_name}</strong> ({req.user_phone})</p>
            <p style={{ fontSize: 13, color: "#666", margin: "4px 0" }}>
              申请角色: {req.requested_role} · 创建时间: {req.created_at}
            </p>
            <p style={{ fontSize: 13, margin: "4px 0" }}>
              原因: {req.reason || "无"}
            </p>
          </div>
          <div style={{ marginBottom: 8 }}>
            <input
              placeholder="审核备注（可选）"
              value={reviewNotes}
              onChange={(e) => setReviewNotes(e.target.value)}
              style={{ width: "100%", padding: "4px 8px", fontSize: 13 }}
            />
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <button
              onClick={() => handleReview(req.id, "approve")}
              style={{ background: "#10a37f", color: "#fff", border: "none", padding: "6px 16px", borderRadius: 4, cursor: "pointer" }}
            >
              批准
            </button>
            <button
              onClick={() => handleReview(req.id, "reject")}
              style={{ background: "#dc2626", color: "#fff", border: "none", padding: "6px 16px", borderRadius: 4, cursor: "pointer" }}
            >
              拒绝
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
