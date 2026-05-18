import { useEffect, useState } from "react";
import { apiJson } from "../api";
import type { UploadDataResponse, DatasetTable, DatasetTablesResponse } from "../types/api";

interface Props {
  token: string;
  onTableListChanged?: () => void;
  datasetId?: string;
  datasetTableId?: string;
}

export function DataManagementPanel({ token, onTableListChanged, datasetId, datasetTableId }: Props) {
  const [showDataMgmt, setShowDataMgmt] = useState(false);
  const [tableList, setTableList] = useState<DatasetTable[]>([]);
  const [dmOp, setDmOp] = useState("insert");
  const [dmTargetTableId, setDmTargetTableId] = useState("");
  const [dmHint, setDmHint] = useState("");
  const [dmFile, setDmFile] = useState<File | null>(null);
  const [dmStatus, setDmStatus] = useState("");
  const [dmResult, setDmResult] = useState<UploadDataResponse | null>(null);

  async function loadTableList() {
    if (!datasetId) {
      setTableList([]);
      return;
    }
    try {
      const j = await apiJson<DatasetTablesResponse>(`/v1/datasets/${datasetId}/tables`, { token });
      const tables = j.tables || [];
      setTableList(tables);
      if (!dmTargetTableId && tables.length > 0) setDmTargetTableId(tables[0].id);
    } catch { /* ignore */ }
  }

  useEffect(() => {
    if (showDataMgmt) void loadTableList();
  }, [showDataMgmt, datasetId]);

  async function doUpload() {
    if (!dmFile) { setDmStatus("请选择文件"); return; }
    if (!datasetId) { setDmStatus("请先选择数据集"); return; }
    if (!dmTargetTableId) { setDmStatus("请选择目标表"); return; }
    setDmStatus("上传中...");
    setDmResult(null);
    try {
      const fd = new FormData();
      fd.append("file", dmFile);
      fd.append("operation", dmOp);
      fd.append("dataset_id", datasetId);
      fd.append("table_id", dmTargetTableId);
      fd.append("ai_hint", dmHint);
      const r = await apiJson<UploadDataResponse>("/v1/data/upload", { method: "POST", token, body: fd });
      setDmResult(r);
      setDmStatus("完成");
      onTableListChanged?.();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setDmStatus(`失败: ${msg}`);
    }
  }

  const opsAllowed = dmOp !== "create_table";

  return (
    <section style={{ marginTop: 16, padding: 12, border: "1px dashed #ccc", borderRadius: 8, background: "#fafafa" }}>
      <button
        type="button"
        onClick={() => { setShowDataMgmt(!showDataMgmt); }}
        style={{ fontWeight: 600, fontSize: 14 }}
      >
        {showDataMgmt ? "收起" : "数据管理"}（文件导入）
      </button>
      {showDataMgmt && (
        <div style={{ marginTop: 12 }}>
          {!datasetId ? (
            <p style={{ fontSize: 13, color: "#888" }}>请先在会话区域选择数据集，再进行数据管理操作</p>
          ) : (
            <>
              <div style={{ display: "flex", gap: 12, marginBottom: 8, flexWrap: "wrap", alignItems: "center" }}>
                <label>
                  操作:
                  <select value={dmOp} onChange={e => setDmOp(e.target.value)} style={{ marginLeft: 4 }}>
                    <option value="insert">导入数据 (INSERT)</option>
                    <option value="update">更新数据 (UPDATE)</option>
                  </select>
                </label>
                <label>
                  目标表:
                  <select value={dmTargetTableId} onChange={e => setDmTargetTableId(e.target.value)} style={{ marginLeft: 4 }}>
                    {tableList.map(t => <option key={t.id} value={t.id}>{t.display_name || t.name}</option>)}
                  </select>
                </label>
              </div>

              {!opsAllowed && (
                <p style={{ fontSize: 13, color: "#888", marginBottom: 8 }}>
                  AI 建表功能已禁用，请前往{""}
                  <a href={`/datasets/${datasetId}/tables`} style={{ color: "#10a37f" }}>数据集表管理</a>
                  {""}手动创建表
                </p>
              )}

              <div style={{ marginBottom: 8 }}>
                <input
                  placeholder="AI 提示（可选，如：第一列是主键）"
                  value={dmHint}
                  onChange={e => setDmHint(e.target.value)}
                  style={{ width: "100%", padding: "4px 8px", fontSize: 13 }}
                />
              </div>

              <div style={{ display: "flex", gap: 8, marginBottom: 8, alignItems: "center" }}>
                <input
                  type="file"
                  accept=".csv,.xlsx,.sql"
                  onChange={e => setDmFile(e.target.files?.[0] || null)}
                />
                <button onClick={doUpload} disabled={!dmFile || !datasetId || !dmTargetTableId}>
                  上传并执行
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
                </div>
              )}
            </>
          )}
        </div>
      )}
    </section>
  );
}
