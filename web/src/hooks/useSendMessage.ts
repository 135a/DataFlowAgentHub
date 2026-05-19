import { useCallback, useEffect, useMemo, useRef } from "react";
import { apiFetch } from "../api";
import { useSSE } from "./useSSE";
import { useSession } from "../contexts/SessionContext";
import { useQuery } from "../contexts/QueryContext";
import { useProgress } from "../contexts/ProgressContext";
import type { RunStep } from "../types/api";

export function useSendMessage() {
  const {
    token,
    sid,
    setSendStatus,
    setSending,
    loadMessages,
  } = useSession();
  const { querySource, mode } = useQuery();
  const {
    initProcessing,
    completeStep,
    finishProcessing,
    updateStepByIndex,
    updateStepDuration,
    setStepStates,
    markAllStepsComplete,
    sendStartRef,
    stepTimestampsRef,
  } = useProgress();

  const stepTimestampsRefLocal = useRef<number[]>([]);

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
      const idx = ({
        nl2sql_node: 0,
        analysis_node: 1,
        chart_node: 2,
        report_node: 3,
      } as Record<string, number>)[step.agent_name];
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
  }), [
    setSendStatus, setSending, setStepStates, finishProcessing, loadMessages,
    updateStepByIndex, updateStepDuration, completeStep, sendStartRef, stepTimestampsRef,
  ]);

  const { startSSE, stopSSE } = useSSE(token, sseCallbacks);

  useEffect(() => {
    return () => stopSSE();
  }, [stopSSE]);

  const send = useCallback(async (text: string) => {
    if (!sid || !token) return;
    setSendStatus("");
    setSending(true);

    // dataset 模式：使用 step 进度跟踪
    if (querySource === "dataset") {
      initProcessing(mode);
    }

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
        if (querySource === "dataset") {
          markAllStepsComplete();
        }
        setSending(false);
        if (querySource === "dataset") {
          setTimeout(() => finishProcessing(), 500);
        }
        await loadMessages();
      } else {
        setSendStatus(`${r.status}`);
        setSending(false);
        if (querySource === "dataset") {
          setTimeout(() => finishProcessing(), 300);
        }
      }
    } catch {
      setSendStatus("网络错误，请重试");
      setSending(false);
      if (querySource === "dataset") {
        setTimeout(() => finishProcessing(), 300);
      }
    }
  }, [
    sid, token, querySource, mode,
    setSendStatus, setSending,
    initProcessing, finishProcessing, markAllStepsComplete,
    loadMessages, startSSE,
  ]);

  return { send };
}
