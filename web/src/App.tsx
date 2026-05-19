import { Link } from "react-router-dom";
import { useSession } from "./contexts/SessionContext";
import { useQuery } from "./contexts/QueryContext";
import { useProgress } from "./contexts/ProgressContext";
import { SessionProvider } from "./contexts/SessionContext";
import { QueryProvider } from "./contexts/QueryContext";
import { ProgressProvider } from "./contexts/ProgressContext";
import { useSendMessage } from "./hooks/useSendMessage";
import { useIsSuperAdmin, useIsDataAdmin, useIsNormalUser } from "./hooks/useRole";
import { SessionSidebar } from "./components/SessionSidebar";
import { QuerySourceSelector } from "./components/QuerySourceSelector";
import { DataManagementPanel } from "./components/DataManagementPanel";
import { ProgressPanel } from "./components/ProgressPanel";
import { ChatInput } from "./components/ChatInput";
import { MessageList } from "./components/MessageList";
import { KnowledgeQueryStatus } from "./components/KnowledgeQueryStatus";
import styles from "./App.module.css";
import type { QuerySource } from "./components/QuerySourceSelector";

function fmtModeDesc(mode: "quick" | "deep"): string {
  return mode === "quick"
    ? "⚡ 快速查询：AI 生成 SQL 并直接执行返回结果，预计等待 1-3 秒，适合简单数据查询"
    : "🔬 深度分析：AI 将依次执行 SQL 生成 → 数据分析 → 图表绘制 → 报告生成，预计等待 5-15 秒，适合复杂数据分析";
}

function AppContent() {
  const {
    token,
    sessions,
    sid,
    messages,
    runSteps,
    sendStatus,
    sending,
    setSid,
    loadSessions,
  } = useSession();
  const {
    mode,
    querySource,
    datasets,
    selectedDatasetId,
    handleSourceChange,
    handleModeChange,
    setSelectedDatasetId,
  } = useQuery();
  const { isProcessing, currentSteps, stepStates, elapsedMs, estimatedRemainingMs } = useProgress();
  const { send } = useSendMessage();

  const isSuperAdmin = useIsSuperAdmin();
  const isDataAdmin = useIsDataAdmin();
  const isNormalUser = useIsNormalUser();

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

      <SessionSidebar
        sessions={sessions}
        sid={sid}
        token={token}
        onSelect={setSid}
        onSessionsChanged={loadSessions}
        datasetId={selectedDatasetId || undefined}
        querySource={querySource}
      />

      {sid && (
        <section>
          <h2>消息</h2>

          <QuerySourceSelector value={querySource} onChange={(v: QuerySource) => handleSourceChange(v)} />

          {querySource === "dataset" && (
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
            </div>
          )}

          {isDataAdmin && querySource === "dataset" && (
            <DataManagementPanel
              token={token}
              onTableListChanged={loadSessions}
              datasetId={selectedDatasetId || undefined}
            />
          )}

          <ChatInput
            onSend={send}
            sending={sending}
            querySource={querySource}
            mode={mode}
            selectedDatasetId={selectedDatasetId}
            onModeChange={handleModeChange}
          />

          {querySource === "dataset" && isProcessing && (
            <ProgressPanel
              steps={currentSteps}
              stepStates={stepStates}
              elapsedMs={elapsedMs}
              estimatedRemainingMs={estimatedRemainingMs}
            />
          )}

          <KnowledgeQueryStatus visible={querySource === "knowledge" && sending} />

          {sendStatus && (
            <p className={`${styles.statusText} ${sendStatus.startsWith("错误") ? styles.statusError : styles.statusInfo}`}>
              {sendStatus}
            </p>
          )}

          {!isProcessing && !sendStatus && !sending && (
            <p className={`${styles.statusText} ${styles.statusHint}`}>
              {querySource === "dataset"
                ? fmtModeDesc(mode)
                : "📚 知识库查询：通过 RAG + LLM 进行问答，预计等待 2-5 秒"}
            </p>
          )}

          <MessageList messages={messages} runSteps={runSteps} />
        </section>
      )}
    </div>
  );
}

export function App() {
  return (
    <SessionProvider>
      <QueryProvider>
        <ProgressProvider>
          <AppContent />
        </ProgressProvider>
      </QueryProvider>
    </SessionProvider>
  );
}
