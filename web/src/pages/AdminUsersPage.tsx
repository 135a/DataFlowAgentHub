import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link, Navigate } from "react-router-dom";
import { apiJson } from "../api";
import { useIsAdmin, useRole } from "../hooks/useRole";
import type { User, UsersResponse } from "../types/api";

export function AdminUsersPage() {
  const token = useMemo(() => localStorage.getItem("token"), []);
  const role = useRole();
  const isAdmin = useIsAdmin();
  const [users, setUsers] = useState<User[]>([]);
  const [status, setStatus] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  const load = async () => {
    try {
      const j = await apiJson<UsersResponse>("/v1/users", { token });
      setUsers(j.users || []);
    } catch { /* keep existing */ }
  };

  useEffect(() => { if (isAdmin) void load(); }, [isAdmin, token]);

  const createUser = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    setStatus("创建中...");
    try {
      await apiJson("/v1/auth/register", {
        method: "POST",
        token,
        body: JSON.stringify({
          name: String(fd.get("name") || ""),
          phone: String(fd.get("phone") || ""),
          password: String(fd.get("password") || ""),
          role: String(fd.get("role") || "read_only_visitor"),
        }),
      });
      setStatus("创建成功");
      setShowCreate(false);
      void load();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "创建失败";
      setStatus(`创建失败: ${msg}`);
    }
    e.currentTarget.reset();
  };

  const changeRole = async (userId: string, newRole: string) => {
    try {
      await apiJson(`/v1/users/${userId}/role`, {
        method: "PUT",
        token,
        body: JSON.stringify({ role: newRole }),
      });
      void load();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "修改失败";
      setStatus(`修改失败: ${msg}`);
    }
  };

  const deleteUser = async (userId: string) => {
    try {
      await apiJson(`/v1/users/${userId}`, { method: "DELETE", token });
      setConfirmDelete(null);
      void load();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "删除失败";
      setStatus(`删除失败: ${msg}`);
    }
  };

  if (!role) return null;
  if (!isAdmin) return <Navigate to="/" replace />;

  return (
    <div style={{ fontFamily: "system-ui", padding: 16, maxWidth: 960, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>用户管理</h1>
        <nav style={{ display: "flex", gap: 16 }}>
          <Link to="/datasets">数据集</Link>
          <Link to="/admin/upgrade-review">审批管理</Link>
          <Link to="/">返回</Link>
        </nav>
      </header>

      <div style={{ margin: "12px 0" }}>
        <button onClick={() => setShowCreate(!showCreate)}>
          {showCreate ? "取消" : "新建用户"}
        </button>
      </div>

      {showCreate && (
        <div style={{ padding: 16, background: "#f5f5f5", borderRadius: 8, marginBottom: 16 }}>
          <form onSubmit={createUser}>
            <div style={{ marginBottom: 8 }}>
              <label style={{ width: 80, display: "inline-block" }}>姓名:</label>
              <input name="name" required />
            </div>
            <div style={{ marginBottom: 8 }}>
              <label style={{ width: 80, display: "inline-block" }}>手机号:</label>
              <input name="phone" required />
            </div>
            <div style={{ marginBottom: 8 }}>
              <label style={{ width: 80, display: "inline-block" }}>密码:</label>
              <input name="password" type="password" required />
            </div>
            <div style={{ marginBottom: 8 }}>
              <label style={{ width: 80, display: "inline-block" }}>角色:</label>
              <select name="role" defaultValue="read_only_visitor">
                <option value="data_admin">data_admin（数据管理员）</option>
                <option value="read_only_visitor">read_only_visitor（只读访客）</option>
              </select>
            </div>
            <button type="submit">创建</button>
          </form>
        </div>
      )}

      {status && (
        <p style={{ fontSize: 13, color: status.includes("失败") ? "#dc2626" : "#10a37f", margin: "8px 0" }}>
          {status}
        </p>
      )}

      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ textAlign: "left", borderBottom: "1px solid #ddd" }}>
            <th>姓名</th><th>手机号</th><th>角色</th><th>创建时间</th><th>操作</th>
          </tr>
        </thead>
        <tbody>
          {users.map(u => (
            <tr key={u.id} style={{ borderBottom: "1px solid #eee" }}>
              <td>{u.name}</td>
              <td>{u.phone || "-"}</td>
              <td>
                {u.role === "super_admin" ? (
                  <span style={{ fontWeight: 600 }}>{u.role}</span>
                ) : (
                  <select
                    value={u.role}
                    onChange={e => changeRole(u.id, e.target.value)}
                    style={{ fontSize: 12 }}
                  >
                    <option value="super_admin">super_admin</option>
                    <option value="data_admin">data_admin</option>
                    <option value="normal_user">normal_user</option>
                    <option value="read_only_visitor">read_only_visitor</option>
                  </select>
                )}
              </td>
              <td>{u.created_at ? new Date(u.created_at).toLocaleString() : "-"}</td>
              <td>
                {u.role !== "super_admin" && (
                  confirmDelete === u.id ? (
                    <span style={{ fontSize: 12 }}>
                      确认删除？
                      <button onClick={() => deleteUser(u.id)} style={{ marginLeft: 4, color: "red" }}>是</button>
                      <button onClick={() => setConfirmDelete(null)} style={{ marginLeft: 4 }}>否</button>
                    </span>
                  ) : (
                    <button
                      onClick={() => setConfirmDelete(u.id)}
                      style={{ fontSize: 11, color: "#dc2626", border: "none", background: "none", cursor: "pointer" }}
                    >
                      删除
                    </button>
                  )
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
