import { useEffect, useMemo, useState, useRef, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { apiJson } from "../api";
import { useWorkspaceId } from "../hooks/useWorkspaceId";
import type { KnowledgeDoc, KnowledgeDocsResponse } from "../types/api";

export function KnowledgePage() {
  const token = useMemo(() => localStorage.getItem("token"), []);
  const [items, setItems] = useState<KnowledgeDoc[]>([]);
  const [status, setStatus] = useState("");
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
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

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] ?? null;
    setSelectedFile(file);
    if (file) {
      setStatus(`已选择: ${file.name}`);
    }
  };

  const upload = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const title = String(fd.get("title") || "").trim();
    const file = selectedFile;
    if (!title) {
      setStatus("标题为必填项");
      return;
    }
    if (!file) {
      setStatus("请选择文件");
      return;
    }
    setStatus("上传中...");
    try {
      const body = new FormData();
      body.append("file", file);
      if (title) body.append("title", title);

      const j = await apiJson<any>(`/v1/workspaces/${workspaceId}/knowledge/docs/upload`, {
        method: "POST",
        token,
        body,
      });
      setStatus(`已提交，任务 ID: ${j.task_id} (${j.doc_type})`);
      setSelectedFile(null);
      if (fileRef.current) fileRef.current.value = "";
      void load();
    } catch {
      setStatus("上传失败，请重试");
    }
    e.currentTarget.reset();
    setSelectedFile(null);
  };

  return (
    <div style={{ fontFamily: "system-ui", padding: 16, maxWidth: 960, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>知识库管理</h1>
        <nav style={{ display: "flex", gap: 16 }}>
          <Link to="/tables">数据库表</Link>
          <Link to="/">返回</Link>
        </nav>
      </header>

      <div style={{ padding: 16, background: "#f5f5f5", borderRadius: 8, margin: "12px 0" }}>
        <form onSubmit={upload} encType="multipart/form-data">
          <div style={{ marginBottom: 8 }}>
            <label style={{ width: 80, display: "inline-block" }}>标题:</label>
            <input name="title" required style={{ width: "60%" }} />
          </div>
          <div style={{ marginBottom: 8 }}>
            <label style={{ width: 80, display: "inline-block" }}>文件:</label>
            <input ref={fileRef} type="file" name="file" required accept=".txt,.doc,.docx,.pdf" onChange={handleFileChange} />
            {selectedFile && <span style={{ marginLeft: 8, fontSize: 12, color: "#888" }}>{selectedFile.name}</span>}
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
