import { useRef, useCallback } from "react";
import { apiJson, getSSEUrl } from "../api";
import type { SSETokenResponse, RunStep } from "../types/api";

interface SSECallbacks {
  onResult: () => void;
  onAgentStep: (step: RunStep) => void;
  onSqlGenerated: () => void;
  onError: (message: string) => void;
}

export function useSSE(token: string, callbacks: SSECallbacks) {
  const esRef = useRef<EventSource | null>(null);
  const retriesRef = useRef(0);
  const startSSERef = useRef<(sessionId: string) => void>(() => {});

  // Hold callbacks in a ref to avoid stale closures
  const cbRef = useRef(callbacks);
  cbRef.current = callbacks;

  const backoffDelay = useCallback((): number => {
    const delay = Math.min(1000 * Math.pow(2, retriesRef.current), 30000);
    retriesRef.current += 1;
    return delay;
  }, []);

  const connectSSE = useCallback((url: string, sessionId: string) => {
    retriesRef.current = 0;
    const es = new EventSource(url);
    esRef.current = es;
    const cb = cbRef.current;

    es.addEventListener("result", () => {
      es.close();
      cb.onResult();
    });

    es.addEventListener("agent_step", (e) => {
      try {
        const step = JSON.parse((e as MessageEvent).data) as RunStep;
        cb.onAgentStep(step);
      } catch { /* ignore parse errors */ }
    });

    es.addEventListener("sql_generated", () => {
      cb.onSqlGenerated();
    });

    es.addEventListener("error", (e) => {
      try {
        const data = JSON.parse((e as MessageEvent).data) as { message?: string };
        cb.onError(data.message || "unknown");
      } catch {
        cb.onError("未知错误");
      }
      es.close();
    });

    es.onerror = () => {
      es.close();
      const delay = backoffDelay();
      setTimeout(() => startSSERef.current(sessionId), delay);
    };
  }, [backoffDelay]);

  const startSSE = useCallback((sessionId: string) => {
    esRef.current?.close();

    apiJson<SSETokenResponse>(`/v1/sessions/${sessionId}/sse-token`, { method: "POST", token })
      .then(j => {
        const url = getSSEUrl(sessionId, j.sse_token);
        connectSSE(url, sessionId);
      })
      .catch(() => {
        const delay = backoffDelay();
        setTimeout(() => startSSERef.current(sessionId), delay);
      });
  }, [token, connectSSE, backoffDelay]);

  startSSERef.current = startSSE;

  const stopSSE = useCallback(() => {
    esRef.current?.close();
  }, []);

  return { startSSE, stopSSE };
}
