import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { apiJson } from "../api";
import { useIsSuperAdmin, useRole } from "../hooks/useRole";
import type { Dataset, DatasetsResponse, CreateDatasetResponse, User, UsersResponse } from "../types/api";

export function DatasetsPage() {
  const navigate = useNavigate();
  const token = localStorage.getItem("token") || "";
  const isSuperAdmin = useIsSuperAdmin();
  const role = useRole();

  const [datasets, setDatasets] = useState<Dataset[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState("");

  // 创建
  const [showCreate, setShowCreate] = useState(false);
  const [createName, setCreateName] = useState("");

  // 重命名
  const [renameId, setRenameId] = useState<string | null>(null);
  const [renameName, setRenameName] = useState("");

  // 删除确认
  const [deleteId, setDeleteId] = useState<string | null>(null);

  // 授权
  const [grantId, setGrantId] = useState<string | null>(null);
  const [users, setUsers] = useState<User[]>([]);
  const [grantUserId, setGrantUserId] = useState("");
  const [grantLevel, setGrantLevel] = useState("read");

  const loadDatasets = useCallback(async () => {
    setLoading(true);
    try {
      const j = await apiJson<DatasetsResponse>("/v1/datasets", { token });
      setDatasets(j.datasets || []);
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => { void loadDatasets(); }, [loadDatasets]);

  async function handleCreate() {
    if (!createName.trim()) return;
    try {
      await apiJson<CreateDatasetResponse>("/v1/datasets", {
        method: "POST", token,
        body: JSON.stringify({ name: createName.trim() }),
      });
      setShowCreate(false);
      setCreateName("");
      void loadDatasets();
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "创建失败");
    }
  }

  async function handleRename(id: string) {
    if (!renameName.trim()) return;
    try {
      await apiJson(`/v1/datasets/${id}`, {
        method: "PUT", token,
        body: JSON.stringify({ name: renameName.trim() }),
      });
      setRenameId(null);
      setRenameName("");
      void loadDatasets();
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "重命名失败");
    }
  }

  async function handleDelete(id: string) {
    try {
      await apiJson(`/v1/datasets/${id}`, { method: "DELETE", token });
      setDeleteId(null);
      void loadDatasets();
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "删除失败");
    }
  }

  async function openGrant(datasetId: string) {
    setGrantId(datasetId);
    try {
      const j = await apiJson<UsersResponse>("/v1/users", { token });
      setUsers(j.users || []);
    } catch { /* ignore */ }
  }

  async function handleGrant() {
    if (!grantUserId || !grantId) return;
    try {
      await apiJson(`/v1/datasets/${grantId}/grant`, {
        method: "POST", token,
        body: JSON.stringify({ user_id: grantUserId, permission_level: grantLevel }),
      });
      setGrantId(null);
      setGrantUserId("");
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "授权失败");
    }
  }

  async function handleRevoke(datasetId: string, userId: string) {
    try {
      await apiJson(`/v1/datasets/${datasetId}/revoke`, {
        method: "POST", token,
        body: JSON.stringify({ user_id: userId }),
      });
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "撤销失败");
    }
  }

  if (loading) return <div style={{ padding: 24 }}>加载中...</div>;

  return (
    <div style={{ maxWidth: 900, margin: "0 auto", padding: 24, fontFamily: "system-ui" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>数据集管理</h1>
        <div style={{ display: "flex", gap: 8 }}>
          <Link to="/" style={{ fontSize: 14 }}>返回聊天</Link>
          {isSuperAdmin && (
            <button onClick={() => setShowCreate(!showCreate)}>
              {showCreate ? "取消" : "创建数据集"}
            </button>
          )}
        </div>
      </div>
      <p style={{ fontSize: 13, color: "#666" }}>当前角色: {role}</p>

      {err && <p style={{ color: "crimson" }}>{err}</p>}

      {showCreate && isSuperAdmin && (
        <div style={{ padding: 12, border: "1px solid #ddd", borderRadius: 8, marginBottom: 16 }}>
          <h3>创建数据集</h3>
          <input
            placeholder="数据集名称"
            value={createName}
            onChange={(e) => setCreateName(e.target.value)}
            style={{ width: "100%", padding: "4px 8px", marginBottom: 8 }}
          />
          <button onClick={handleCreate}>确认创建</button>
        </div>
      )}

      {datasets.length === 0 && !loading && (
        <p style={{ color: "#888" }}>暂无数据集</p>
      )}

      {datasets.map((ds) => (
        <div
          key={ds.id}
          style={{
            border: "1px solid #e0e0e0", borderRadius: 8, padding: 16, marginBottom: 12,
            background: "#fafafa", cursor: "pointer",
          }}
          onClick={() => navigate(`/datasets/${ds.id}/tables`)}
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <div>
              <h3 style={{ margin: 0 }}>{ds.name}</h3>
              <p style={{ fontSize: 12, color: "#888", margin: "4px 0" }}>
                MySQL: {ds.mysql_database} · 状态: {ds.status} · 创建时间: {ds.created_at}
              </p>
            </div>
            {isSuperAdmin && (
              <div style={{ display: "flex", gap: 4 }} onClick={(e) => e.stopPropagation()}>
                <button
                  onClick={() => { setRenameId(ds.id); setRenameName(ds.name); }}
                  style={{ fontSize: 12 }}
                >
                  重命名
                </button>
                <button onClick={() => openGrant(ds.id)} style={{ fontSize: 12 }}>
                  授权
                </button>
                <button
                  onClick={() => setDeleteId(ds.id)}
                  style={{ fontSize: 12, color: "crimson" }}
                >
                  删除
                </button>
              </div>
            )}
          </div>

          {renameId === ds.id && (
            <div style={{ marginTop: 8, padding: 8, background: "#fff", borderRadius: 6 }}>
              <input
                value={renameName}
                onChange={(e) => setRenameName(e.target.value)}
                style={{ width: "100%", padding: "4px 8px", marginBottom: 4 }}
              />
              <div style={{ display: "flex", gap: 4 }}>
                <button onClick={() => handleRename(ds.id)}>确认</button>
                <button onClick={() => setRenameId(null)}>取消</button>
              </div>
            </div>
          )}

          {deleteId === ds.id && (
            <div style={{ marginTop: 8, padding: 8, background: "#fff0f0", borderRadius: 6 }}>
              <p style={{ fontSize: 13, margin: "0 0 4px" }}>确认删除数据集 "{ds.name}"？此操作不可撤销。</p>
              <div style={{ display: "flex", gap: 4 }}>
                <button onClick={() => handleDelete(ds.id)} style={{ color: "crimson" }}>确认删除</button>
                <button onClick={() => setDeleteId(null)}>取消</button>
              </div>
            </div>
          )}

          {grantId === ds.id && (
            <div style={{ marginTop: 8, padding: 8, background: "#fff", borderRadius: 6 }} onClick={(e) => e.stopPropagation()}>
              <h4 style={{ margin: "0 0 8px" }}>授权用户</h4>
              <select value={grantUserId} onChange={(e) => setGrantUserId(e.target.value)} style={{ width: "100%", marginBottom: 4 }}>
                <option value="">选择用户</option>
                {users.filter(u => u.role !== "super_admin").map((u) => (
                  <option key={u.id} value={u.id}>{u.name} ({u.role})</option>
                ))}
              </select>
              <select value={grantLevel} onChange={(e) => setGrantLevel(e.target.value)} style={{ width: "100%", marginBottom: 4 }}>
                <option value="read">只读</option>
                <option value="write">读写</option>
                <option value="admin">管理</option>
              </select>
              <div style={{ display: "flex", gap: 4 }}>
                <button onClick={handleGrant}>确认授权</button>
                <button onClick={() => setGrantId(null)}>取消</button>
              </div>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
