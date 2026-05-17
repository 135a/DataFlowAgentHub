import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { Link } from "react-router-dom";
import { apiFetch, apiJson, getSSEUrl } from "./api";
import { useIsAdmin, useIsOperator } from "./hooks/useRole";
import { ResultTable } from "./ResultTable";
import { ChartView } from "./components/ChartView";
import { ModeSelector } from "./components/ModeSelector";
import { ProgressPanel } from "./components/ProgressPanel";
import type { StepDef, StepState } from "./components/ProgressPanel";
import type {
  Session,
  ApiMessage,
  MessageContent,
  RunStep,
  SessionsResponse,
  MessagesResponse,
  CreateSessionResponse,
  SSETokenResponse,
  PostMessageResponse,
} from "./types/api";

type QueryMode = "quick" | "deep";

const QUICK_STEPS: StepDef[] = [
  { name: "SQL 生成", weight: 0.7 },
  { name: "执行查询", weight: 0.3 },
];

const DEEP_STEPS: StepDef[] = [
  { name: "SQL 生成", weight: 0.15 },
  { name: "数据分析", weight: 0.25 },
  { name: "图表绘制", weight: 0.40 },
  { name: "报告生成", weight: 0.20 },
];

const AGENT_STEP_MAP: Record<string, number> = {
  nl2sql_node: 0,
  analysis_node: 1,
  chart_node: 2,
  report_node: 3,
};

const STEP_DEFAULTS: Record<string, number> = {
  "SQL 生成": 2000,
  "执行查询": 300,
  "数据分析": 3500,
  "图表绘制": 4500,
  "报告生成": 2000,
};

function loadStepHistory(): Record<string, number[]> {
  try {
    return JSON.parse(localStorage.getItem("stepHistory") || "{}");
  } catch {
    return {};
  }
}

function getAvgDuration(history: Record<string, number[]>, stepName: string): number {
  const durations = history[stepName];
  if (!durations || durations.length === 0) return STEP_DEFAULTS[stepName] || 2000;
  return durations.reduce((a, b) => a + b, 0) / durations.length;
}

function calcInitialEstimate(steps: StepDef[], history: Record<string, number[]>): number {
  return steps.reduce((total, s) => total + getAvgDuration(history, s.name), 0);
}

function makeWaitingStates(n: number): StepState[] {
  return Array.from({ length: n }, (_, i) => ({
    status: i === 0 ? "running" as const : "waiting" as const,
    durationMs: 0,
  }));
}

function fmtModeDesc(mode: QueryMode): string {
  return mode === "quick"
    ? "⚡ 快速查询：AI 生成 SQL 并直接执行返回结果，预计等待 1-3 秒，适合简单数据查询"
    : "🔬 深度分析：AI 将依次执行 SQL 生成 → 数据分析 → 图表绘制 → 报告生成，预计等待 5-15 秒，适合复杂数据分析";
}

export function App() {
  const token = useMemo(() => localStorage.getItem("token"), []);
  const isAdmin = useIsAdmin();
  const isOperator = useIsOperator();
  const [sessions, setSessions] = useState<Session[]>([]);
  // ---- data management state ----
  const [showDataMgmt, setShowDataMgmt] = useState(false);
  const [tableList, setTableList] = useState<string[]>([]);
  const [dmOp, setDmOp] = useState("insert");
  const [dmTarget, setDmTarget] = useState("");
  const [dmHint, setDmHint] = useState("");
  const [dmFile, setDmFile] = useState<File | null>(null);
  const [dmStatus, setDmStatus] = useState("");
  const [dmResult, setDmResult] = useState<any>(null);
  // AI suggest state
  const [dmAiDesc, setDmAiDesc] = useState("");
  const [dmAiSuggestion, setDmAiSuggestion] = useState<any>(null);
  const [dmAiLoading, setDmAiLoading] = useState(false);
  const [sid, setSid] = useState<string | null>(null);
  const [messages, setMessages] = useState<ApiMessage[]>([]);
  const [runSteps, setRunSteps] = useState<RunStep[]>([]);
  const [sendStatus, setSendStatus] = useState<string>("");
  const [mode, setMode] = useState<QueryMode>(() => {
    return (localStorage.getItem("queryMode") as QueryMode) || "deep";
  });
  const [sending, setSending] = useState(false);
  const esRef = useRef<EventSource | null>(null);
  const sseRetries = useRef(0);

  // ---- progress tracking state ----
  const [isProcessing, setIsProcessing] = useState(false);
  const [currentSteps, setCurrentSteps] = useState<StepDef[]>([]);
  const [stepStates, setStepStates] = useState<StepState[]>([]);
  const [elapsedMs, setElapsedMs] = useState(0);
  const [estimatedRemainingMs, setEstimatedRemainingMs] = useState<number | null>(null);
  const timerRef = useRef<number | null>(null);
  const sendStartRef = useRef(0);
  const stepTimestampsRef = useRef<number[]>([]);

  function stopTimer() {
    if (timerRef.current !== null) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
  }

  function startTimer() {
    stopTimer();
    sendStartRef.current = Date.now();
    setElapsedMs(0);
    timerRef.current = window.setInterval(() => {
      setElapsedMs(Date.now() - sendStartRef.current);
    }, 200);
  }

  function saveStepHistory(durations: number[]) {
    try {
      const history = loadStepHistory();
      currentSteps.forEach((s, i) => {
        const arr = history[s.name] || [];
        arr.push(durations[i]);
        history[s.name] = arr;
      });
      localStorage.setItem("stepHistory", JSON.stringify(history));
    } catch { /* ignore storage errors */ }
  }

  function updateStepByIndex(idx: number, status: StepState["status"]) {
    setStepStates(prev => {
      const next = [...prev];
      next[idx] = { ...next[idx], status };
      return next;
    });
  }

  function updateStepDuration(idx: number) {
    const elapsed = Date.now() - sendStartRef.current;
    setStepStates(prev => {
      const next = [...prev];
      next[idx] = { ...next[idx], durationMs: elapsed };
      return next;
    });
  }

  function completeStep(idx: number) {
    updateStepDuration(idx);
    updateStepByIndex(idx, "completed");
    // 启动下一步
    setStepStates(prev => {
      const next = [...prev];
      if (idx + 1 < next.length) {
        next[idx + 1] = { ...next[idx + 1], status: "running", durationMs: 0 };
      }
      return next;
    });
    stepTimestampsRef.current[idx] = Date.now() - sendStartRef.current;
    // 更新预估
    setStepStates(prev => {
      const completedDurations = prev
        .map((st, i) => ({ st, weight: currentSteps[i]?.weight ?? 0, idx: i }))
        .filter(x => x.st.status === "completed" && x.st.durationMs > 0);
      if (completedDurations.length > 0) {
        const totalWeight = completedDurations.reduce((s, x) => s + x.weight, 0);
        const totalTime = completedDurations.reduce((s, x) => s + x.st.durationMs, 0);
        const avgPerWeight = totalTime / totalWeight;
        const remainingWeight = currentSteps.slice(idx + 1).reduce((s, x) => s + x.weight, 0);
        setEstimatedRemainingMs(avgPerWeight * remainingWeight);
      }
      return prev;
    });
  }

  function finishProcessing() {
    stopTimer();
    // 保存耗时记录
    const durations = currentSteps.map((_, i) => stepTimestampsRef.current[i] || 0);
    saveStepHistory(durations);
    setIsProcessing(false);
  }

  // ---- end progress tracking ----

  useEffect(() => {
    void (async () => {
      try {
        const j = await apiJson<SessionsResponse>("/v1/sessions", { token: token! });
        setSessions(j.sessions || []);
      } catch { /* sessions load failure is non-fatal */ }
    })();
  }, [token]);

  const loadMessages = useCallback(async () => {
    if (!sid || !token) return;
    try {
      const j = await apiJson<MessagesResponse>(`/v1/sessions/${sid}/messages`, { token });
      setMessages(j.messages || []);
      setRunSteps(j.run_steps || []);
    } catch {
      setMessages([]);
      setRunSteps([]);
    }
  }, [sid, token]);

  useEffect(() => {
    void loadMessages();
  }, [loadMessages]);

  useEffect(() => {
    return () => {
      esRef.current?.close();
    };
  }, []);

  function handleModeChange(newMode: QueryMode) {
    setMode(newMode);
    localStorage.setItem("queryMode", newMode);
  }

  function startSSE(sessionId: string) {
    esRef.current?.close();

    apiJson<SSETokenResponse>(`/v1/sessions/${sessionId}/sse-token`, { method: "POST", token })
      .then(j => {
        const url = getSSEUrl(sessionId, j.sse_token);
        connectSSE(url, sessionId);
      })
      .catch(() => {
        const delay = backoffDelay();
        setTimeout(() => startSSE(sessionId), delay);
      });
  }

  function backoffDelay(): number {
    const delay = Math.min(1000 * Math.pow(2, sseRetries.current), 30000);
    sseRetries.current += 1;
    return delay;
  }

  function connectSSE(url: string, sessionId: string) {
    sseRetries.current = 0;
    const es = new EventSource(url);
    esRef.current = es;

    es.addEventListener("result", () => {
      es.close();
      setSendStatus("完成");
      setSending(false);
      // 标记剩余步骤为完成
      setStepStates(prev => {
        const next = [...prev];
        for (let i = 0; i < next.length; i++) {
          if (next[i].status !== "completed") {
            next[i] = { ...next[i], status: "completed", durationMs: Date.now() - sendStartRef.current };
            stepTimestampsRef.current[i] = Date.now() - sendStartRef.current;
          }
        }
        return next;
      });
      setTimeout(() => finishProcessing(), 500);
      loadMessages();
    });

    es.addEventListener("agent_step", (e) => {
      try {
        const step = JSON.parse((e as MessageEvent).data) as RunStep;
        const idx = AGENT_STEP_MAP[step.agent_name];
        if (idx === undefined) return;
        if (step.status === "running") {
          updateStepDuration(idx);
          setStepStates(prev => {
            const next = [...prev];
            next[idx] = { ...next[idx], status: "running", durationMs: 0 };
            return next;
          });
        } else if (step.status === "succeeded") {
          completeStep(idx);
        } else if (step.status === "failed") {
          updateStepDuration(idx);
          updateStepByIndex(idx, "error");
        }
      } catch { /* ignore parse errors */ }
    });

    es.addEventListener("sql_generated", () => {
      // 同步路径：步骤0（SQL生成）完成
      completeStep(0);
    });

    es.addEventListener("error", (e) => {
      try {
        const data = JSON.parse((e as MessageEvent).data) as { message?: string };
        setSendStatus(`错误: ${data.message || "unknown"}`);
      } catch {
        setSendStatus("错误");
      }
      es.close();
      updateStepByIndex(0, "error");
      setTimeout(() => finishProcessing(), 300);
      loadMessages();
    });

    es.onerror = () => {
      es.close();
      const delay = backoffDelay();
      setTimeout(() => startSSE(sessionId), delay);
    };
  }

  async function send(text: string) {
    if (!sid || !token) return;
    setSendStatus("");
    setSending(true);

    // 初始化进度追踪
    const steps = mode === "deep" ? DEEP_STEPS : QUICK_STEPS;
    setCurrentSteps(steps);
    const initialStates = makeWaitingStates(steps.length);
    setStepStates(initialStates);
    stepTimestampsRef.current = [];
    const history = loadStepHistory();
    const initialEstimate = calcInitialEstimate(steps, history);
    setEstimatedRemainingMs(initialEstimate);
    setIsProcessing(true);
    startTimer();

    const workflow = mode === "deep" ? "agent_pipeline" : "auto";
    try {
      const r = await apiFetch(`/v1/sessions/${sid}/messages`, {
        method: "POST",
        token,
        body: JSON.stringify({ text, workflow }),
      });

      if (r.status === 202) {
        // 异步路径：SSE 事件驱动进度
        startSSE(sid);
      } else if (r.ok) {
        // 同步路径：直接完成所有步骤
        for (let i = 0; i < steps.length; i++) {
          stepTimestampsRef.current[i] = Date.now() - sendStartRef.current;
        }
        setStepStates(prev => prev.map(st => ({ ...st, status: "completed" as const })));
        setSending(false);
        setTimeout(() => finishProcessing(), 500);
        await loadMessages();
      } else {
        setSendStatus(`${r.status}`);
        setSending(false);
        setTimeout(() => finishProcessing(), 300);
      }
    } catch {
      setSendStatus("网络错误，请重试");
      setSending(false);
      setTimeout(() => finishProcessing(), 300);
    }
  }

  // ---- data management functions ----

  async function loadTableList() {
    try {
      const j = await apiJson<{ tables: { name: string }[] }>("/v1/schema/tables", { token: token! });
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
      const r = await apiJson<any>("/v1/data/upload", { method: "POST", token: token!, body: fd });
      setDmResult(r);
      setDmStatus("完成");
    } catch (err: any) {
      setDmStatus(`失败: ${err.message || err}`);
    }
  }

  async function doSuggestTable() {
    if (!dmAiDesc.trim()) { setDmStatus("请输入表描述"); return; }
    setDmAiLoading(true);
    setDmAiSuggestion(null);
    setDmStatus("");
    try {
      const j = await apiJson<any>("/v1/data/suggest-table", {
        method: "POST",
        token: token!,
        body: JSON.stringify({ description: dmAiDesc, ai_hint: dmHint }),
      });
      setDmAiSuggestion(j);
    } catch (err: any) {
      setDmStatus(`AI 建议失败: ${err.message || err}`);
    } finally {
      setDmAiLoading(false);
    }
  }

  async function doCreateTable(ddl: string) {
    setDmStatus("建表中...");
    try {
      const j = await apiJson<any>("/v1/data/create-table", {
        method: "POST",
        token: token!,
        body: JSON.stringify({ ddl }),
      });
      setDmResult(j);
      setDmAiSuggestion(null);
      setDmStatus("建表成功");
      void loadTableList();
    } catch (err: any) {
      setDmStatus(`建表失败: ${err.message || err}`);
    }
  }

  return (
    <div style={{ fontFamily: "system-ui", padding: 16, maxWidth: 960, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1 style={{ margin: 0 }}>DataFlowAgentHub</h1>
        <nav style={{ display: "flex", gap: 16, alignItems: "center" }}>
          <Link to="/data-sources" style={{ fontSize: 14 }}>数据源</Link>
          <Link to="/tables" style={{ fontSize: 14 }}>数据库表</Link>
          <Link to="/knowledge" style={{ fontSize: 14 }}>知识库</Link>
          {isAdmin && <Link to="/admin/users" style={{ fontSize: 14 }}>用户管理</Link>}
          <Link to="/login" onClick={() => localStorage.removeItem("token")}>
            退出
          </Link>
        </nav>
      </header>
      <p style={{ color: "#555", fontSize: 14 }}>
        MVP：消息通过 REST + SSE 实时推送；异步任务通过 EventSource 订阅{" "}
        <code>/v1/sessions/&lt;id&gt;/stream?token=&lt;sse_token&gt;</code>
      </p>
      {isOperator && (
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
              {/* operation & target */}
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

              {/* AI hint */}
              <div style={{ marginBottom: 8 }}>
                <input
                  placeholder="AI 提示（可选，如：第一列是主键）"
                  value={dmHint}
                  onChange={e => setDmHint(e.target.value)}
                  style={{ width: "100%", padding: "4px 8px", fontSize: 13 }}
                />
              </div>

              {/* AI create table */}
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

              {/* file upload */}
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

              {/* status & result */}
              {dmStatus && (
                <p style={{ fontSize: 13, color: dmStatus.includes("失败") ? "#dc2626" : "#10a37f", margin: "4px 0" }}>
                  {dmStatus}
                </p>
              )}
              {dmResult && (
                <div style={{ padding: 8, background: "#fff", borderRadius: 6, fontSize: 13 }}>
                  {dmResult.ok !== undefined && (
                    <p>状态: {dmResult.ok ? "成功" : "失败"} {dmResult.rows_affected !== undefined ? `· 影响 ${dmResult.rows_affected} 行` : ""}</p>
                  )}
                  {dmResult.error && <p style={{ color: "#dc2626" }}>{dmResult.error}</p>}
                  {dmResult.message && <p>{dmResult.message}</p>}
                  {dmResult.ddl && <pre style={{ fontSize: 11, background: "#eee", padding: 4, borderRadius: 4, overflowX: "auto" }}>{dmResult.ddl}</pre>}
                </div>
              )}
            </div>
          )}
        </section>
      )}

      <section style={{ marginTop: 16 }}>
        <h2>会话</h2>
        <button
          type="button"
          onClick={async () => {
            try {
              const j = await apiJson<CreateSessionResponse>("/v1/sessions", {
                method: "POST",
                token: token!,
                body: JSON.stringify({ title: "新会话" }),
              });
              setSessions((s) => [{ id: j.id, title: j.title }, ...s]);
              setSid(j.id);
            } catch { /* ignore */ }
          }}
        >
          新建
        </button>
        <ul>
          {sessions.map((s) => (
            <li key={s.id}>
              <button
                type="button"
                onClick={() => setSid(s.id)}
                style={{ fontWeight: sid === s.id ? "bold" : "normal" }}
              >
                {s.title}
              </button>
            </li>
          ))}
        </ul>
      </section>
      {sid && (
        <section>
          <h2>消息</h2>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const fd = new FormData(e.currentTarget);
              const t = String(fd.get("t") || "");
              void send(t);
              e.currentTarget.reset();
            }}
          >
            <ModeSelector mode={mode} onChange={handleModeChange} />
            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
              <input
                name="t"
                placeholder="例如：how many rows in demo_sales"
                style={{ flex: 1, padding: "6px 10px", fontSize: 14 }}
                disabled={sending}
              />
              <button type="submit" style={{ padding: "6px 16px" }} disabled={sending}>
                {sending ? "处理中..." : "发送"}
              </button>
            </div>
          </form>

          {isProcessing ? (
            <ProgressPanel
              steps={currentSteps}
              stepStates={stepStates}
              elapsedMs={elapsedMs}
              estimatedRemainingMs={estimatedRemainingMs}
            />
          ) : sendStatus ? (
            <p style={{ fontSize: 13, color: sendStatus.startsWith("错误") ? "#dc2626" : "#444", margin: "8px 0 0" }}>
              {sendStatus}
            </p>
          ) : (
            <p style={{ fontSize: 13, color: "#6b7280", margin: "8px 0 0" }}>
              {fmtModeDesc(mode)}
            </p>
          )}

          <div style={{ marginTop: 16, display: "flex", flexDirection: "column", gap: 12 }}>
            {messages.map((m) => (
              <MessageBlock key={m.id} msg={m} />
            ))}

            {runSteps.length > 0 && (
              <div style={{ padding: 12, background: "#f8f9fa", borderRadius: 8, fontSize: 13, border: "1px dashed #ccc" }}>
                <strong>中间执行步骤:</strong>
                <ul style={{ paddingLeft: 20, margin: "8px 0 0" }}>
                  {runSteps.map((step, idx) => (
                    <li key={idx}>
                      <span style={{color: step.status === 'running' ? '#10a37f' : step.status === 'failed' ? 'red' : '#555'}}>
                        [{step.agent_name}]
                      </span>{" "}
                      {step.status}: {step.output_summary || step.input_summary}
                      {step.error_message && <span style={{color:'red'}}> (Error: {step.error_message})</span>}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  );
}

function MessageBlock({ msg }: { msg: ApiMessage }) {
  const isUser = msg.role === "user";
  const bubble: CSSProperties = {
    alignSelf: isUser ? "flex-end" : "flex-start",
    maxWidth: "92%",
    padding: "10px 14px",
    borderRadius: 12,
    background: isUser ? "#1a5fb4" : "#eef1f5",
    color: isUser ? "#fff" : "#111",
    fontSize: 14,
  };

  return (
    <div style={bubble}>
      <div style={{ fontSize: 11, opacity: 0.75, marginBottom: 6 }}>
        {msg.role} · {msg.created_at}
      </div>
      <MessageBody content={msg.content} />
    </div>
  );
}

function MessageBody({ content }: { content: MessageContent }) {
  if (content === null || content === undefined) {
    return <span style={{ opacity: 0.7 }}>（空）</span>;
  }
  if (typeof content !== "object") {
    return <pre style={{ margin: 0, whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{String(content)}</pre>;
  }
  const c = content as unknown as Record<string, unknown>;

  if (typeof c.text === "string") {
    return <p style={{ margin: 0, whiteSpace: "pre-wrap" }}>{c.text}</p>;
  }

  if (typeof c.error === "string") {
    return (
      <div>
        <strong>错误</strong>
        <pre style={{ margin: "8px 0 0", whiteSpace: "pre-wrap" }}>{c.error}</pre>
        {typeof c.code === "string" ? <code style={{ fontSize: 12 }}>{c.code}</code> : null}
      </div>
    );
  }

  const sql = typeof c.sql === "string" ? c.sql : null;
  const rawRows = c.rows;
  const rows = Array.isArray(rawRows) ? (rawRows as Record<string, unknown>[]) : null;

  if (sql !== null && rows) {
    const notes = c.notes;
    const hasNumeric = rows.length > 0 && Object.values(rows[0] || {}).some(v => typeof v === "number");
    return <SqlResultBlock sql={sql} rows={rows} hasNumeric={hasNumeric} notes={notes} />;
  }

  if (c.final_report) {
    const reportObj = c.final_report as Record<string, unknown>;
    const finalReportText = String(reportObj.final_report || "");
    const runId = c.run_id as string | undefined;
    return (
      <div>
        <strong style={{ fontSize: 12 }}>生成报告</strong>
        <pre style={{ margin: "8px 0", whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{finalReportText}</pre>
        {runId && (
          <div style={{ marginTop: 8 }}>
            <a href={`/api/v1/runs/${runId}/report`} target="_blank" rel="noreferrer" style={{
              display: "inline-block", padding: "4px 12px", background: "#10a37f", color: "#fff", textDecoration: "none", borderRadius: 4, fontSize: 13
            }}>下载 Excel 报告</a>
          </div>
        )}
      </div>
    );
  }

  return (
    <pre style={{ margin: 0, fontSize: 12, overflowX: "auto" }}>{JSON.stringify(content, null, 2)}</pre>
  );
}

function SqlResultBlock({ sql, rows, hasNumeric, notes }: {
  sql: string;
  rows: Record<string, unknown>[];
  hasNumeric: boolean;
  notes: unknown;
}) {
  const [view, setView] = useState<"table" | "chart">("table");

  return (
    <div>
      <div style={{ marginBottom: 8 }}>
        <strong style={{ fontSize: 12 }}>SQL</strong>
        <pre
          style={{
            margin: "4px 0 0",
            padding: 8,
            background: "rgba(0,0,0,0.06)",
            borderRadius: 6,
            fontSize: 12,
            overflowX: "auto",
          }}
        >
          {sql}
        </pre>
      </div>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 4 }}>
        <strong style={{ fontSize: 12 }}>结果</strong>
        {hasNumeric && (
          <button
            type="button"
            onClick={() => setView(v => v === "table" ? "chart" : "table")}
            style={{
              padding: "2px 10px",
              fontSize: 11,
              cursor: "pointer",
              border: "1px solid #ccc",
              borderRadius: 4,
              background: "#fff",
            }}
          >
            {view === "table" ? "图表" : "表格"}
          </button>
        )}
      </div>
      {view === "table" ? (
        <ResultTable rows={rows} />
      ) : (
        <ChartView rows={rows} />
      )}
      {notes != null &&
      (Array.isArray(notes) ? notes.length > 0 : String(notes).length > 0) ? (
        <p style={{ fontSize: 12, margin: "8px 0 0", opacity: 0.85 }}>
          自检：{Array.isArray(notes) ? notes.map(String).join("；") : String(notes)}
        </p>
      ) : null}
    </div>
  );
}

