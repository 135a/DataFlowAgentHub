import type { Session } from "../types/api";

interface Props {
  sessions: Session[];
  sid: string | null;
  token: string;
  onSelect: (id: string) => void;
  onSessionsChanged: () => void;
}

export function SessionSidebar({ sessions, sid, token, onSelect, onSessionsChanged }: Props) {
  async function createSession() {
    try {
      const r = await fetch("/v1/sessions", {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ title: "新会话" }),
      });
      if (!r.ok) return;
      const j = await r.json() as { id: string; title: string };
      onSessionsChanged();
      onSelect(j.id);
    } catch { /* ignore */ }
  }

  return (
    <section style={{ marginTop: 16 }}>
      <h2>会话</h2>
      <button type="button" onClick={createSession}>
        新建
      </button>
      <ul>
        {sessions.map((s) => (
          <li key={s.id}>
            <button
              type="button"
              onClick={() => onSelect(s.id)}
              style={{ fontWeight: sid === s.id ? "bold" : "normal" }}
            >
              {s.title}
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
