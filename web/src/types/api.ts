// ── Session ──
export interface Session {
  id: string;
  title: string;
}

// ── Message Content Variants ──
export interface TextContent {
  text: string;
}

export interface SqlResultContent {
  sql: string;
  rows: Record<string, unknown>[];
  notes?: string[];
}

export interface ErrorContent {
  error: string;
  code?: string;
}

export interface ReportContent {
  final_report: {
    final_report: string;
  };
  run_id?: string;
}

export type MessageContent = TextContent | SqlResultContent | ErrorContent | ReportContent;

// ── Message ──
export interface ApiMessage {
  id: string;
  role: string;
  content: MessageContent;
  created_at: string;
}

// ── Run Step ──
export interface RunStep {
  agent_name: string;
  status: string;
  input_summary?: string;
  output_summary?: string;
  error_message?: string;
}

// ── Data Source ──
export interface DataSource {
  id: string;
  name: string;
  kind: string;
  host: string;
  port: number;
  database: string;
  has_password: boolean;
}

// ── Knowledge Doc ──
export interface KnowledgeDoc {
  id: string;
  title: string;
  doc_type: string;
  status: string;
  chunk_count?: number;
  created_at: string;
}

// ── Auth ──
export interface LoginResponse {
  access_token: string;
}

export interface SSETokenResponse {
  sse_token: string;
}

// ── API Response Wrappers ──
export interface SessionsResponse {
  sessions: Session[];
}

export interface MessagesResponse {
  messages: ApiMessage[];
  run_steps: RunStep[];
}

export interface DataSourcesResponse {
  items: DataSource[];
}

export interface KnowledgeDocsResponse {
  docs: KnowledgeDoc[];
}

export interface CreateSessionResponse {
  id: string;
  title: string;
}

export interface UploadKnowledgeResponse {
  id: string;
  task_id: string;
  status: string;
}

export interface PostMessageResponse {
  task_id: string;
}

// ── Data Management ──
export interface UploadDataResponse {
  ok?: boolean;
  rows_affected?: number;
  error?: string;
  message?: string;
  ddl?: string;
}

export interface SuggestTableResponse {
  table_name: string;
  explanation: string;
  ddl: string;
}

export interface CreateTableResponse {
  ok?: boolean;
  message?: string;
  error?: string;
  ddl?: string;
}

export interface TableListResponse {
  tables: { name: string }[];
}
