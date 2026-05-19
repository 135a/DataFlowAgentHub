import {
  createContext,
  useContext,
  useState,
  useEffect,
  type ReactNode,
} from "react";
import { apiJson } from "../api";
import type { QuerySource } from "../components/QuerySourceSelector";
import type { Dataset, DatasetsResponse } from "../types/api";

type QueryMode = "quick" | "deep";

interface QueryContextValue {
  mode: QueryMode;
  querySource: QuerySource;
  datasets: Dataset[];
  selectedDatasetId: string;
  handleModeChange: (m: QueryMode) => void;
  handleSourceChange: (s: QuerySource) => void;
  setSelectedDatasetId: (id: string) => void;
}

const QueryContext = createContext<QueryContextValue | null>(null);

export function useQuery(): QueryContextValue {
  const ctx = useContext(QueryContext);
  if (!ctx) throw new Error("useQuery must be used within QueryProvider");
  return ctx;
}

export function QueryProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<QueryMode>(() => {
    return (localStorage.getItem("queryMode") as QueryMode) || "deep";
  });

  const [querySource, setQuerySource] = useState<QuerySource>(() => {
    return (localStorage.getItem("querySource") as QuerySource) || "knowledge";
  });

  const [datasets, setDatasets] = useState<Dataset[]>([]);
  const [selectedDatasetId, setSelectedDatasetId] = useState("");

  const token = localStorage.getItem("token") || "";

  // Load datasets
  useEffect(() => {
    if (!token) return;
    apiJson<DatasetsResponse>("/v1/datasets", { token })
      .then(j => setDatasets(j.datasets || []))
      .catch(() => {});
  }, [token]);

  function handleModeChange(newMode: QueryMode) {
    setMode(newMode);
    localStorage.setItem("queryMode", newMode);
  }

  function handleSourceChange(newSource: QuerySource) {
    setQuerySource(newSource);
    localStorage.setItem("querySource", newSource);
    if (newSource === "knowledge") {
      setSelectedDatasetId("");
    }
  }

  return (
    <QueryContext.Provider
      value={{
        mode,
        querySource,
        datasets,
        selectedDatasetId,
        handleModeChange,
        handleSourceChange,
        setSelectedDatasetId,
      }}
    >
      {children}
    </QueryContext.Provider>
  );
}
