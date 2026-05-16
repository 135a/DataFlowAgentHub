import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { Link, useNavigate } from "react-router-dom";
import { apiFetch, getSSEUrl } from "./api";
import { ResultTable } from "./ResultTable";

type Session = { id: string; title: string };

type ApiMessage = {
  id: string;
  role: string;
  content: unknown;
  created_at: string;
};

export function App() {
  const token = useMemo(() => localStorage.getItem("token"), []);
  const navigate = useNavigate();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [sid, setSid] = useState<string | null>(null);
  const [messages, setMessages] = useState<ApiMessage[]>([]);
  const [runSteps, setRunSteps] = useState<any[]>([]);
  const [sendStatus, setSendStatus] = useState<string>("");
  const [deepAnalysis, setDeepAnalysis] = useState(false);
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    void (async () => {
      const r = await apiFetch("/v1/sessions", { token: token! });
      const j = await r.json();
      setSessions(j.sessions || []);
    })();
  }, [token]);

  const loadMessages = useCallback(async () => {
    if (!sid || !token) return;
    const r = await apiFetch(`/v1/sessions/${sid}/messages`, { token });
    if (!r.ok) {
      setMessages([]);
      setRunSteps([]);
      return;
    }
    const j = await r.json();
    setMessages(j.messages || []);
    setRunSteps(j.run_steps || []);
  }, [sid, token]);

  useEffect(() => {
    void loadMessages();
  }, [loadMessages]);

  // Cleanup EventSource on unmount
  useEffect(() => {
    return () => {
      esRef.current?.close();
    };
  }, []);

  function startSSE(sessionId: string) {
    // Close existing connection
    esRef.current?.close();

    // Fetch SSE token
    apiFetch(`/v1/sessions/${sessionId}/sse-token`, { method: "POST", token })
      .then(r => r.json())
      .then(j => {
        const url = getSSEUrl(sessionId, j.sse_token);
        connectSSE(url, sessionId);
      })
      .catch(() => {
        // Fallback: retry token fetch after 3s
        setTimeout(() => startSSE(sessionId), 3000);
      });
  }

  function connectSSE(url: string, sessionId: string) {
    const es = new EventSource(url);
    esRef.current = es;

    es.addEventListener("result", (e) => {
      es.close();
      setSendStatus("完成");
      loadMessages();
    });

    es.addEventListener("agent_step", (e) => {
      try {
        const step = JSON.parse(e.data);
        setSendStatus(`[${step.agent_name}] ${step.status}`);
      } catch { /* ignore parse errors */ }
    });

    es.addEventListener("approval_required", () => {
      setSendStatus("需要审批");
    });

    es.addEventListener("error", (e) => {
      try {
        const data = JSON.parse(e.data);
        setSendStatus(`错误: ${data.message || "unknown"}`);
      } catch {
        setSendStatus("错误");
      }
      es.close();
      loadMessages();
    });

    es.onerror = () => {
      es.close();
      // Reconnect after 3 seconds
      setTimeout(() => startSSE(sessionId), 3000);
    };
  }

  async function send(text: string) {
    if (!sid || !token) return;
    setSendStatus("发送中...");
    const workflow = deepAnalysis ? "agent_pipeline" : "auto";
    const r = await apiFetch(`/v1/sessions/${sid}/messages`, {
      method: "POST",
      token,
      body: JSON.stringify({ text, workflow }),
    });
    const j = await r.json().catch(() => ({}));

    if (r.status === 202) {
      setSendStatus(`任务处理中 (task: ${j.task_id})，SSE 监听中...`);
      startSSE(sid);
    } else if (r.ok) {
      setSendStatus("完成");
      await loadMessages();
    } else {
      setSendStatus(`${r.status} ${typeof j === "string" ? j : JSON.stringify(j)}`);
    }
  }

  return (
    <div style={{ fontFamily: "system-ui", padding: 16, maxWidth: 960, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1 style={{ margin: 0 }}>DataFlowAgentHub</h1>
        <nav style={{ display: "flex", gap: 16, alignItems: "center" }}>
          <Link to="/data-sources" style={{ fontSize: 14 }}>数据源</Link>
          <Link to="/knowledge" style={{ fontSize: 14 }}>知识库</Link>
          <Link to="/login" onClick={() => localStorage.removeItem("token")}>
            退出
          </Link>
        </nav>
      </header>
      <p style={{ color: "#555", fontSize: 14 }}>
        MVP：消息通过 REST + SSE 实时推送；异步任务通过 EventSource 订阅{" "}
        <code>/v1/sessions/&lt;id&gt;/stream?token=&lt;sse_token&gt;</code>
      </p>
      <section style={{ marginTop: 16 }}>
        <h2>会话</h2>
        <button
          type="button"
          onClick={async () => {
            const r = await apiFetch("/v1/sessions", {
              method: "POST",
              token: token!,
              body: JSON.stringify({ title: "新会话" }),
            });
            const j = await r.json();
            setSessions((s) => [{ id: j.id, title: j.title }, ...s]);
            setSid(j.id);
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
            <input name="t" placeholder="例如：how many rows in demo_sales" style={{ width: "60%" }} />
            <label style={{ marginLeft: 12, fontSize: 14, cursor: "pointer" }}>
              <input
                type="checkbox"
                checked={deepAnalysis}
                onChange={(e) => setDeepAnalysis(e.target.checked)}
              />
              {" "}深度分析
            </label>
            <button type="submit" style={{ marginLeft: 8 }}>发送</button>
          </form>
          {sendStatus ? (
            <p style={{ fontSize: 13, color: "#444", margin: "8px 0 0" }}>
              {sendStatus}
            </p>
          ) : null}
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
      <section style={{ marginTop: 24 }}>
        <h2>审批（需 operator/admin）</h2>
        <Approvals token={token!} />
      </section>
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

function MessageBody({ content }: { content: unknown }) {
  if (content === null || content === undefined) {
    return <span style={{ opacity: 0.7 }}>（空）</span>;
  }
  if (typeof content !== "object") {
    return <pre style={{ margin: 0, whiteSpace: "pre-wrap", wordBreak: "break-word" }}>{String(content)}</pre>;
  }
  const c = content as Record<string, unknown>;

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
        <strong style={{ fontSize: 12 }}>结果</strong>
        <ResultTable rows={rows} />
        {notes != null &&
        (Array.isArray(notes) ? notes.length > 0 : String(notes).length > 0) ? (
          <p style={{ fontSize: 12, margin: "8px 0 0", opacity: 0.85 }}>
            自检：{Array.isArray(notes) ? notes.map(String).join("；") : String(notes)}
          </p>
        ) : null}
      </div>
    );
  }

  if (c.final_report) {
    const reportObj = c.final_report as any;
    const finalReportText = reportObj.final_report || "";
    const runId = c.run_id;
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

function Approvals({ token }: { token: string }) {
  const [items, setItems] = useState<any[]>([]);
  const load = async () => {
    const r = await apiFetch("/v1/approvals", { token });
    if (!r.ok) {
      setItems([]);
      return;
    }
    const j = await r.json();
    setItems(j.items || []);
  };
  useEffect(() => {
    void load();
  }, [token]);
  return (
    <div>
      <button type="button" onClick={() => void load()}>
        刷新
      </button>
      <ul>
        {items.map((it) => (
          <li key={it.id}>
            {it.action_type}{" "}
            <button
              type="button"
              onClick={async () => {
                await apiFetch(`/v1/approvals/${it.id}/decide`, {
                  method: "POST",
                  token,
                  body: JSON.stringify({ decision: "approve" }),
                });
                void load();
              }}
            >
              批准
            </button>
            <button
              type="button"
              onClick={async () => {
                await apiFetch(`/v1/approvals/${it.id}/decide`, {
                  method: "POST",
                  token,
                  body: JSON.stringify({ decision: "reject" }),
                });
                void load();
              }}
            >
              驳回
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
