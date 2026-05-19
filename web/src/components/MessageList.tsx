import { MessageBlock, RunStepsPanel } from "./ChatPanel";
import type { ApiMessage, RunStep } from "../types/api";
import styles from "../App.module.css";

interface MessageListProps {
  messages: ApiMessage[];
  runSteps: RunStep[];
}

export function MessageList({ messages, runSteps }: MessageListProps) {
  if (messages.length === 0) return null;
  return (
    <div className={styles.messagesContainer}>
      {messages.map((m) => (
        <MessageBlock key={m.id} msg={m} />
      ))}
      <RunStepsPanel steps={runSteps} />
    </div>
  );
}
