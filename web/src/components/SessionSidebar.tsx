import type { Session } from "../types/api";
import styles from "./SessionSidebar.module.css";

interface Props {
  sessions: Session[];
  sid: string | null;
  token: string;
  onSelect: (id: string) => void;
  onSessionsChanged: () => void;
  datasetId?: string;
  datasetTableId?: string;
}

export function SessionSidebar({ sessions, sid, token, onSelect, onSessionsChanged, datasetId, datasetTableId }: Props) {
  async function createSession() {
    try {
      const body: Record<string, string> = { title: "新会话" };
      if (datasetId) body.dataset_id = datasetId;
      if (datasetTableId) body.dataset_table_id = datasetTableId;
      const r = await fetch("/v1/sessions", {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!r.ok) return;
      const j = await r.json() as { id: string; title: string };
      onSessionsChanged();
      onSelect(j.id);
    } catch { /* ignore */ }
  }

  return (
    <section className={styles.sidebar}>
      <h2 className={styles.heading}>会话</h2>
      <button type="button" onClick={createSession} className={styles.createBtn}>
        新建
      </button>
      <ul className={styles.sessionList}>
        {sessions.map((s) => (
          <li key={s.id} className={styles.sessionItem}>
            <button
              type="button"
              onClick={() => onSelect(s.id)}
              className={sid === s.id ? styles.sessionBtnBold : styles.sessionBtn}
            >
              {s.title}
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
