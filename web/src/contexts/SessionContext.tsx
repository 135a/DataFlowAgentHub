import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useMemo,
  type ReactNode,
} from "react";
import { apiJson } from "../api";
import type {
  Session,
  ApiMessage,
  RunStep,
  SessionsResponse,
  MessagesResponse,
} from "../types/api";

interface SessionContextValue {
  token: string;
  sessions: Session[];
  sid: string | null;
  messages: ApiMessage[];
  runSteps: RunStep[];
  sendStatus: string;
  sending: boolean;
  setSid: (id: string | null) => void;
  setSendStatus: (s: string) => void;
  setSending: (v: boolean) => void;
  setMessages: (msgs: ApiMessage[]) => void;
  setRunSteps: (steps: RunStep[]) => void;
  loadSessions: () => Promise<void>;
  loadMessages: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue | null>(null);

export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession must be used within SessionProvider");
  return ctx;
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const token = useMemo(() => localStorage.getItem("token") || "", []);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [sid, setSid] = useState<string | null>(null);
  const [messages, setMessages] = useState<ApiMessage[]>([]);
  const [runSteps, setRunSteps] = useState<RunStep[]>([]);
  const [sendStatus, setSendStatus] = useState("");
  const [sending, setSending] = useState(false);

  const loadSessions = useCallback(async () => {
    try {
      const j = await apiJson<SessionsResponse>("/v1/sessions", { token });
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

  return (
    <SessionContext.Provider
      value={{
        token,
        sessions,
        sid,
        messages,
        runSteps,
        sendStatus,
        sending,
        setSid,
        setSendStatus,
        setSending,
        setMessages,
        setRunSteps,
        loadSessions,
        loadMessages,
      }}
    >
      {children}
    </SessionContext.Provider>
  );
}
