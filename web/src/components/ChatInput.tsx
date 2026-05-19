import { type FormEvent } from "react";
import { ModeSelector } from "./ModeSelector";
import styles from "../App.module.css";

interface ChatInputProps {
  onSend: (text: string) => void;
  sending: boolean;
  querySource: "dataset" | "knowledge";
  mode: "quick" | "deep";
  selectedDatasetId: string;
  onModeChange: (m: "quick" | "deep") => void;
}

export function ChatInput({
  onSend,
  sending,
  querySource,
  mode,
  selectedDatasetId,
  onModeChange,
}: ChatInputProps) {
  return (
    <form
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const fd = new FormData(e.currentTarget);
        const t = String(fd.get("t") || "");
        if (!t.trim()) return;
        onSend(t);
        e.currentTarget.reset();
      }}
    >
      {querySource === "dataset" && (
        <ModeSelector mode={mode} onChange={onModeChange} />
      )}
      <div className={styles.inputRow}>
        <input
          name="t"
          placeholder={
            querySource === "dataset"
              ? "例如：how many rows in demo_sales"
              : "请输入知识库查询问题"
          }
          className={styles.textInput}
          disabled={sending}
        />
        <button
          type="submit"
          className={styles.sendButton}
          disabled={sending || (querySource === "dataset" && !selectedDatasetId)}
        >
          {sending ? "处理中..." : "发送"}
        </button>
      </div>
    </form>
  );
}
