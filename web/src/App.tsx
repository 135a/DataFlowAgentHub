import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch, apiJson } from "./api";
import { useIsSuperAdmin, useIsDataAdmin, useIsNormalUser } from "./hooks/useRole";
import { useSSE } from "./hooks/useSSE";
import { ModeSelector } from "./components/ModeSelector";
import { ProgressPanel } from "./components/ProgressPanel";
import { DataManagementPanel } from "./components/DataManagementPanel";
import { SessionSidebar } from "./components/SessionSidebar";
import { MessageBlock, RunStepsPanel } from "./components/ChatPanel";
import type { StepDef, StepState } from "./components/ProgressPanel";
import type {
  Session,
  ApiMessage,
  RunStep,
  SessionsResponse,
  MessagesResponse,
  Dataset,
  DatasetsResponse,
  DatasetTable,
  DatasetTablesResponse,
} from "./types/api";
import styles from "./App.module.css";

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
  const isSuperAdmin = useIsSuperAdmin();
  const isDataAdmin = useIsDataAdmin();
  const isNormalUser = useIsNormalUser();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [sid, setSid] = useState<string | null>(null);
  const [messages, setMessages] = useState<ApiMessage[]>([]);
  const [runSteps, setRunSteps] = useState<RunStep[]>([]);
  const [sendStatus, setSendStatus] = useState<string>("");
  const [mode, setMode] = useState<QueryMode>(() => {
    return (localStorage.getItem("queryMode") as QueryMode) || "deep";
  });
  const [sending, setSending] = useState(false);

  // ---- dataset / table selector state ----
  const [datasets, setDatasets] = useState<Dataset[]>([]);
  const [selectedDatasetId, setSelectedDatasetId] = useState("");
  const [datasetTables, setDatasetTables] = useState<DatasetTable[]>([]);
  const [selectedTableId, setSelectedTableId] = useState("");

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
    setStepStates(prev => {
      const next = [...prev];
      if (idx + 1 < next.length) {
        next[idx + 1] = { ...next[idx + 1], status: "running", durationMs: 0 };
      }
      return next;
    });
    stepTimestampsRef.current[idx] = Date.now() - sendStartRef.current;
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
    const durations = currentSteps.map((_, i) => stepTimestampsRef.current[i] || 0);
    saveStepHistory(durations);
    setIsProcessing(false);
  }

  // ---- end progress tracking ----

  const loadSessions = useCallback(async () => {
    try {
      const j = await apiJson<SessionsResponse>("/v1/sessions", { token: token! });
      setSessions(j.sessions || []);
    } catch { /* sessions load failure is non-fatal */ }
  }, [token]);

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

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

  // ---- load datasets ----
  useEffect(() => {
    if (!token) return;
    apiJson<DatasetsResponse>("/v1/datasets", { token })
      .then(j => setDatasets(j.datasets || []))
      .catch(() => {});
  }, [token]);

  // ---- load tables when dataset changes ----
  useEffect(() => {
    if (!selectedDatasetId || !token) {
      setDatasetTables([]);
      setSelectedTableId("");
      return;
    }
    apiJson<DatasetTablesResponse>(`/v1/datasets/${selectedDatasetId}/tables`, { token })
      .then(j => setDatasetTables(j.tables || []))
      .catch(() => setDatasetTables([]));
  }, [selectedDatasetId, token]);

  function handleModeChange(newMode: QueryMode) {
    setMode(newMode);
    localStorage.setItem("queryMode", newMode);
  }

  // --- SSE hook ---
  const sseCallbacks = useMemo(() => ({
    onResult: () => {
      setSendStatus("完成");
      setSending(false);
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
    },
    onAgentStep: (step: RunStep) => {
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
    },
    onSqlGenerated: () => {
      completeStep(0);
    },
    onError: (message: string) => {
      setSendStatus(`错误: ${message}`);
      updateStepByIndex(0, "error");
      setTimeout(() => finishProcessing(), 300);
      loadMessages();
    },
  }), [loadMessages]);

  const { startSSE, stopSSE } = useSSE(token!, sseCallbacks);

  useEffect(() => {
    return () => stopSSE();
  }, [stopSSE]);

  async function send(text: string) {
    if (!sid || !token) return;
    setSendStatus("");
    setSending(true);

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
        startSSE(sid);
      } else if (r.ok) {
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

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <h1 className={styles.headerTitle}>DataFlowAgentHub</h1>
        <nav className={styles.nav}>
          <Link to="/datasets" className={styles.navLink}>数据集</Link>
          <Link to="/knowledge" className={styles.navLink}>知识库</Link>
          {isSuperAdmin && <Link to="/admin/users" className={styles.navLink}>用户管理</Link>}
          {isSuperAdmin && <Link to="/admin/upgrade-review" className={styles.navLink}>审批管理</Link>}
          {isNormalUser && <Link to="/upgrade-request" className={styles.navLink}>升级申请</Link>}
          <Link to="/login" onClick={() => localStorage.removeItem("token")}>
            退出
          </Link>
        </nav>
      </header>
      <p style={{ color: "#555", fontSize: 14 }}>
        MVP：消息通过 REST + SSE 实时推送；异步任务通过 EventSource 订阅{" "}
        <code>/v1/sessions/&lt;id&gt;/stream?token=&lt;sse_token&gt;</code>
      </p>

      {isDataAdmin && (
        <DataManagementPanel
          token={token!}
          onTableListChanged={loadSessions}
          datasetId={selectedDatasetId || undefined}
          datasetTableId={selectedTableId || undefined}
        />
      )}

      <SessionSidebar
        sessions={sessions}
        sid={sid}
        token={token!}
        onSelect={setSid}
        onSessionsChanged={loadSessions}
        datasetId={selectedDatasetId || undefined}
        datasetTableId={selectedTableId || undefined}
      />

      {sid && (
        <section>
          <h2>消息</h2>

          {/* 数据集 / 数据表 选择器 */}
          <div style={{ display: "flex", gap: 8, marginBottom: 8, alignItems: "center" }}>
            <label style={{ fontSize: 13 }}>
              数据集:
              <select
                value={selectedDatasetId}
                onChange={(e) => setSelectedDatasetId(e.target.value)}
                style={{ marginLeft: 4, padding: "2px 4px" }}
              >
                <option value="">请选择数据集</option>
                {datasets.map((ds) => (
                  <option key={ds.id} value={ds.id}>{ds.name}</option>
                ))}
              </select>
            </label>
            <label style={{ fontSize: 13 }}>
              数据表:
              <select
                value={selectedTableId}
                onChange={(e) => setSelectedTableId(e.target.value)}
                style={{ marginLeft: 4, padding: "2px 4px" }}
                disabled={!selectedDatasetId}
              >
                <option value="">请选择数据表</option>
                {datasetTables.map((t) => (
                  <option key={t.id} value={t.id}>{t.display_name || t.name}</option>
                ))}
              </select>
            </label>
          </div>

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
            <div className={styles.inputRow}>
              <input
                name="t"
                placeholder="例如：how many rows in demo_sales"
                className={styles.textInput}
                disabled={sending}
              />
              <button type="submit" className={styles.sendButton} disabled={sending}>
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
            <p className={`${styles.statusText} ${sendStatus.startsWith("错误") ? styles.statusError : styles.statusInfo}`}>
              {sendStatus}
            </p>
          ) : (
            <p className={`${styles.statusText} ${styles.statusHint}`}>
              {fmtModeDesc(mode)}
            </p>
          )}

          <div className={styles.messagesContainer}>
            {messages.map((m) => (
              <MessageBlock key={m.id} msg={m} />
            ))}
            <RunStepsPanel steps={runSteps} />
          </div>
        </section>
      )}
    </div>
  );
}
