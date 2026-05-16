import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { apiJson } from "../api";
import { useWorkspaceId } from "../hooks/useWorkspaceId";
import type { KnowledgeDoc, KnowledgeDocsResponse, UploadKnowledgeResponse } from "../types/api";

export function KnowledgePage() {
  const token = useMemo(() => localStorage.getItem("token"), []);
  const [items, setItems] = useState<KnowledgeDoc[]>([]);
  const [status, setStatus] = useState("");
  const workspaceId = useWorkspaceId();

  const load = async () => {
    try {
      const j = await apiJson<KnowledgeDocsResponse>(`/v1/workspaces/${workspaceId}/knowledge/docs`, { token });
      setItems(j.docs || []);
    } catch {
      // keep existing items
    }
  };

  useEffect(() => { void load(); }, [token]);

  const upload = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    setStatus("上传中...");
    try {
      const j = await apiJson<UploadKnowledgeResponse>(`/v1/workspaces/${workspaceId}/knowledge/docs`, {
        method: "POST",
        token,
        body: JSON.stringify({
          title: String(fd.get("title") || ""),
          content: String(fd.get("content") || ""),
        }),
      });
      setStatus(`已提交，任务 ID: ${j.task_id}`);
      void load();
    } catch {
      setStatus("上传失败，请重试");
    }
    e.currentTarget.reset();
  };

  return (
    <div style={{ fontFamily: "system-ui", padding: 16, maxWidth: 960, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>知识库管理</h1>
        <Link to="/">返回</Link>
      </header>

      <div style={{ padding: 16, background: "#f5f5f5", borderRadius: 8, margin: "12px 0" }}>
        <form onSubmit={upload}>
          <div style={{ marginBottom: 8 }}>
            <label style={{ width: 80, display: "inline-block" }}>标题:</label>
            <input name="title" required style={{ width: "60%" }} />
          </div>
          <div style={{ marginBottom: 8 }}>
            <label style={{ width: 80, display: "inline-block", verticalAlign: "top" }}>内容:</label>
            <textarea name="content" required rows={5} style={{ width: "60%" }} />
          </div>
          <button type="submit">上传</button>
          {status && <span style={{ marginLeft: 12, fontSize: 13 }}>{status}</span>}
        </form>
      </div>

      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ textAlign: "left", borderBottom: "1px solid #ddd" }}>
            <th>标题</th><th>类型</th><th>状态</th><th>分块数</th><th>上传时间</th>
          </tr>
        </thead>
        <tbody>
          {items.map((it) => (
            <tr key={it.id} style={{ borderBottom: "1px solid #eee" }}>
              <td>{it.title}</td>
              <td>{it.doc_type}</td>
              <td>{it.status}</td>
              <td>{it.chunk_count ?? "-"}</td>
              <td>{it.created_at ? new Date(it.created_at).toLocaleString() : "-"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
