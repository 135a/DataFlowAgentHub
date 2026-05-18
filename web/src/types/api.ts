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

// ── Data Source (legacy) ──
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

// ── API Response Wrappers (legacy) ──
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

// ── Data Management (legacy) ──
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

// ── Dataset (v2) ──
export interface Dataset {
  id: string;
  name: string;
  mysql_database: string;
  status: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface DatasetsResponse {
  datasets: Dataset[];
}

export interface CreateDatasetResponse {
  id: string;
  name: string;
  mysql_database: string;
}

// ── Dataset Table (v2) ──
export interface DatasetTable {
  id: string;
  dataset_id: string;
  name: string;
  display_name?: string;
  mysql_table_name: string;
  status: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface DatasetTablesResponse {
  tables: DatasetTable[];
}

export interface CreateTableResponseV2 {
  id: string;
  name: string;
  mysql_table_name: string;
  fields_count: number;
}

// ── Table Field (v2) ──
export interface TableField {
  id: string;
  table_id: string;
  name: string;
  display_name?: string;
  field_type: string;
  field_length: number;
  is_nullable: boolean;
  ordinal_position: number;
}

export interface TableDetailResponse {
  table: DatasetTable;
  fields: TableField[];
}

export interface FieldsResponse {
  fields: TableField[];
}

// ── Permission ──
export interface PermissionGrantRequest {
  user_id: string;
  permission_level: string;
}

export interface PermissionRevokeRequest {
  user_id: string;
}

// ── Upgrade Request ──
export interface UpgradeRequest {
  id: string;
  user_id: string;
  user_name: string;
  user_phone: string;
  requested_role: string;
  reason: string;
  status: string;
  created_at: string;
}

export interface UpgradeRequestsResponse {
  requests: UpgradeRequest[];
}

// ── User (4-role) ──
export interface User {
  id: string;
  name: string;
  phone: string;
  role: string;
  created_at: string;
}

export interface UsersResponse {
  users: User[];
}
