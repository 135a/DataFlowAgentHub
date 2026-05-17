import { useState } from "react";
import { apiJson } from "../api";
import type { UploadDataResponse, SuggestTableResponse, CreateTableResponse, TableListResponse } from "../types/api";

interface Props {
  token: string;
  onTableListChanged?: () => void;
}

export function DataManagementPanel({ token, onTableListChanged }: Props) {
  const [showDataMgmt, setShowDataMgmt] = useState(false);
  const [tableList, setTableList] = useState<string[]>([]);
  const [dmOp, setDmOp] = useState("insert");
  const [dmTarget, setDmTarget] = useState("");
  const [dmHint, setDmHint] = useState("");
  const [dmFile, setDmFile] = useState<File | null>(null);
  const [dmStatus, setDmStatus] = useState("");
  const [dmResult, setDmResult] = useState<UploadDataResponse | CreateTableResponse | null>(null);
  const [dmAiDesc, setDmAiDesc] = useState("");
  const [dmAiSuggestion, setDmAiSuggestion] = useState<SuggestTableResponse | null>(null);
  const [dmAiLoading, setDmAiLoading] = useState(false);

  async function loadTableList() {
    try {
      const j = await apiJson<TableListResponse>("/v1/schema/tables", { token });
      const names = (j.tables || []).map(t => t.name);
      setTableList(names);
      if (!dmTarget && names.length > 0) setDmTarget(names[0]);
    } catch { /* ignore */ }
  }

  async function doUpload() {
    if (!dmFile) { setDmStatus("请选择文件"); return; }
    if (dmOp !== "create_table" && !dmTarget) { setDmStatus("请选择目标表"); return; }
    setDmStatus("上传中...");
    setDmResult(null);
    try {
      const fd = new FormData();
      fd.append("file", dmFile);
      fd.append("operation", dmOp);
      fd.append("target_table", dmTarget);
      fd.append("ai_hint", dmHint);
      const r = await apiJson<UploadDataResponse>("/v1/data/upload", { method: "POST", token, body: fd });
      setDmResult(r);
      setDmStatus("完成");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setDmStatus(`失败: ${msg}`);
    }
  }

  async function doSuggestTable() {
    if (!dmAiDesc.trim()) { setDmStatus("请输入表描述"); return; }
    setDmAiLoading(true);
    setDmAiSuggestion(null);
    setDmStatus("");
    try {
      const j = await apiJson<SuggestTableResponse>("/v1/data/suggest-table", {
        method: "POST",
        token,
        body: JSON.stringify({ description: dmAiDesc, ai_hint: dmHint }),
      });
      setDmAiSuggestion(j);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setDmStatus(`AI 建议失败: ${msg}`);
    } finally {
      setDmAiLoading(false);
    }
  }

  async function doCreateTable(ddl: string) {
    setDmStatus("建表中...");
    try {
      const j = await apiJson<CreateTableResponse>("/v1/data/create-table", {
        method: "POST",
        token,
        body: JSON.stringify({ ddl }),
      });
      setDmResult(j);
      setDmAiSuggestion(null);
      setDmStatus("建表成功");
      void loadTableList();
      onTableListChanged?.();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setDmStatus(`建表失败: ${msg}`);
    }
  }

  return (
    <section style={{ marginTop: 16, padding: 12, border: "1px dashed #ccc", borderRadius: 8, background: "#fafafa" }}>
      <button
        type="button"
        onClick={() => { setShowDataMgmt(!showDataMgmt); if (!showDataMgmt) void loadTableList(); }}
        style={{ fontWeight: 600, fontSize: 14 }}
      >
        {showDataMgmt ? "收起" : "数据管理"}（文件导入 / 建表）
      </button>
      {showDataMgmt && (
        <div style={{ marginTop: 12 }}>
          <div style={{ display: "flex", gap: 12, marginBottom: 8, flexWrap: "wrap", alignItems: "center" }}>
            <label>
              操作:
              <select value={dmOp} onChange={e => setDmOp(e.target.value)} style={{ marginLeft: 4 }}>
                <option value="insert">导入数据 (INSERT)</option>
                <option value="update">更新数据 (UPDATE)</option>
                <option value="create_table">创建新表</option>
              </select>
            </label>
            {dmOp !== "create_table" && (
              <label>
                目标表:
                <select value={dmTarget} onChange={e => setDmTarget(e.target.value)} style={{ marginLeft: 4 }}>
                  {tableList.map(t => <option key={t} value={t}>{t}</option>)}
                </select>
              </label>
            )}
          </div>

          <div style={{ marginBottom: 8 }}>
            <input
              placeholder="AI 提示（可选，如：第一列是主键）"
              value={dmHint}
              onChange={e => setDmHint(e.target.value)}
              style={{ width: "100%", padding: "4px 8px", fontSize: 13 }}
            />
          </div>

          {dmOp === "create_table" && (
            <div style={{ marginBottom: 8, padding: 8, background: "#fff", borderRadius: 6 }}>
              <p style={{ fontSize: 13, margin: "0 0 6px" }}>AI 智能建表：用自然语言描述你需要的表结构</p>
              <div style={{ display: "flex", gap: 8 }}>
                <input
                  placeholder="例如：需要一个客户信息表，包含姓名、手机号、邮箱、注册日期"
                  value={dmAiDesc}
                  onChange={e => setDmAiDesc(e.target.value)}
                  style={{ flex: 1, padding: "4px 8px", fontSize: 13 }}
                />
                <button onClick={doSuggestTable} disabled={dmAiLoading} style={{ whiteSpace: "nowrap" }}>
                  {dmAiLoading ? "生成中..." : "AI 建议"}
                </button>
              </div>
              {dmAiSuggestion && (
                <div style={{ marginTop: 8, padding: 8, background: "#eef6ff", borderRadius: 6, fontSize: 13 }}>
                  <p><strong>建议表名:</strong> {dmAiSuggestion.table_name}</p>
                  <p><strong>设计说明:</strong> {dmAiSuggestion.explanation}</p>
                  <pre style={{ margin: "8px 0", padding: 6, background: "#ddd", borderRadius: 4, fontSize: 12, overflowX: "auto" }}>
                    {dmAiSuggestion.ddl}
                  </pre>
                  <div style={{ display: "flex", gap: 8 }}>
                    <button onClick={() => doCreateTable(dmAiSuggestion.ddl)} style={{ background: "#10a37f", color: "#fff", border: "none", padding: "4px 12px", borderRadius: 4, cursor: "pointer" }}>
                      确认建表
                    </button>
                    <button onClick={() => setDmAiSuggestion(null)}>取消</button>
                  </div>
                </div>
              )}
            </div>
          )}

          <div style={{ display: "flex", gap: 8, marginBottom: 8, alignItems: "center" }}>
            <input
              type="file"
              accept=".csv,.xlsx,.sql"
              onChange={e => setDmFile(e.target.files?.[0] || null)}
            />
            <button onClick={doUpload} disabled={!dmFile}>
              {dmOp === "create_table" ? "上传 SQL 文件" : "上传并执行"}
            </button>
          </div>
          {dmFile && <p style={{ fontSize: 12, color: "#888", margin: "0 0 8px" }}>已选择: {dmFile.name}</p>}

          {dmStatus && (
            <p style={{ fontSize: 13, color: dmStatus.includes("失败") ? "#dc2626" : "#10a37f", margin: "4px 0" }}>
              {dmStatus}
            </p>
          )}
          {dmResult && (
            <div style={{ padding: 8, background: "#fff", borderRadius: 6, fontSize: 13 }}>
              {"ok" in dmResult && dmResult.ok !== undefined && (
                <p>状态: {dmResult.ok ? "成功" : "失败"} {"rows_affected" in dmResult && dmResult.rows_affected !== undefined ? `· 影响 ${dmResult.rows_affected} 行` : ""}</p>
              )}
              {"error" in dmResult && dmResult.error && <p style={{ color: "#dc2626" }}>{dmResult.error}</p>}
              {"message" in dmResult && dmResult.message && <p>{dmResult.message}</p>}
              {"ddl" in dmResult && dmResult.ddl && <pre style={{ fontSize: 11, background: "#eee", padding: 4, borderRadius: 4, overflowX: "auto" }}>{dmResult.ddl}</pre>}
            </div>
          )}
        </div>
      )}
    </section>
  );
}
