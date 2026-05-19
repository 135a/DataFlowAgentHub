import {
  createContext,
  useContext,
  useState,
  useRef,
  useCallback,
  type ReactNode,
} from "react";
import type { StepDef, StepState } from "../components/ProgressPanel";

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
    status: (i === 0 ? "running" : "waiting") as const,
    durationMs: 0,
  }));
}

type QueryMode = "quick" | "deep";

interface ProgressContextValue {
  isProcessing: boolean;
  currentSteps: StepDef[];
  stepStates: StepState[];
  elapsedMs: number;
  estimatedRemainingMs: number | null;
  sendStartRef: React.MutableRefObject<number>;
  stepTimestampsRef: React.MutableRefObject<number[]>;
  initProcessing: (mode: QueryMode) => void;
  completeStep: (idx: number) => void;
  finishProcessing: () => void;
  setStepStates: React.Dispatch<React.SetStateAction<StepState[]>>;
  updateStepByIndex: (idx: number, status: StepState["status"]) => void;
  updateStepDuration: (idx: number) => void;
  markAllStepsComplete: () => void;
}

const ProgressContext = createContext<ProgressContextValue | null>(null);

export function useProgress(): ProgressContextValue {
  const ctx = useContext(ProgressContext);
  if (!ctx) throw new Error("useProgress must be used within ProgressProvider");
  return ctx;
}

export function ProgressProvider({ children }: { children: ReactNode }) {
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

  const saveStepHistory = useCallback(
    (durations: number[]) => {
      try {
        const history = loadStepHistory();
        currentSteps.forEach((s, i) => {
          const arr = history[s.name] || [];
          arr.push(durations[i]);
          history[s.name] = arr;
        });
        localStorage.setItem("stepHistory", JSON.stringify(history));
      } catch { /* ignore storage errors */ }
    },
    [currentSteps],
  );

  function initProcessing(mode: QueryMode) {
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

  function markAllStepsComplete() {
    const steps = currentSteps.length > 0 ? currentSteps : DEEP_STEPS;
    for (let i = 0; i < steps.length; i++) {
      stepTimestampsRef.current[i] = Date.now() - sendStartRef.current;
    }
    setStepStates(prev => prev.map(st => ({ ...st, status: "completed" as const })));
  }

  function finishProcessing() {
    stopTimer();
    const durations = currentSteps.map((_, i) => stepTimestampsRef.current[i] || 0);
    saveStepHistory(durations);
    setIsProcessing(false);
  }

  return (
    <ProgressContext.Provider
      value={{
        isProcessing,
        currentSteps,
        stepStates,
        elapsedMs,
        estimatedRemainingMs,
        sendStartRef,
        stepTimestampsRef,
        initProcessing,
        completeStep,
        finishProcessing,
        setStepStates,
        updateStepByIndex,
        updateStepDuration,
        markAllStepsComplete,
      }}
    >
      {children}
    </ProgressContext.Provider>
  );
}
