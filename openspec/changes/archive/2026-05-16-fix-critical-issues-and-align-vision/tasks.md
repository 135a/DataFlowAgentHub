## 1. 编译阻断 Bug 修复

- [x] 1.1 在 `config.Config` 中添加 `InternalHMACSecret string` 字段（env: `HUB_INTERNAL_HMAC_SECRET`），添加启动时必填校验
- [x] 1.2 在 `internal/handlers/tasks.go` 的 import 块中补上 `ssebus` 包引用
- [x] 1.3 删除 `internal/handlers/knowledge.go` 中未使用的 `"io"` import
- [x] 1.4 运行 `go build ./...` 验证编译通过

## 2. 运行时阻断 Bug 修复

- [x] 2.1 修复 `Dockerfile.ai`：将 `COPY services/ai/` 改为拷贝整个 `services/ai/` 目录（含 orchestrator/、agents/、rag/）
- [x] 2.2 重新运行 `make gen` 生成完整 gRPC 桩代码（Go + Python），确保 `RunAgentPipeline` 出现在客户端接口中
- [x] 2.3 在 `internal/worker/nl2sql.go` 中添加 `RunAgentPipeline` 客户端封装方法
- [x] 2.4 修复 `services/ai/orchestrator/consumer.py` 的 `headers` 变量作用域问题（在 try 块前初始化 `headers = {}`）
- [x] 2.5 将 `migrations/005_async_tasks.sql`、`006_knowledge_docs.sql`、`007_agent_run_steps.sql` 移入 `internal/migrate/`，重命名为 `002_async_tasks.sql`、`003_knowledge_docs.sql`、`004_agent_run_steps.sql`
- [x] 2.6 实现 `schema_migrations` 版本追踪表及迁移跳过逻辑（`internal/migrate/migrate.go`）
- [x] 2.7 在 `Dockerfile.api` 中将 Go 版本更新至 1.25（匹配 `go.mod`）
- [x] 2.8 在 `requirements.txt` 中补全 `tabulate` 依赖
- [x] 2.9 运行 `docker compose -f deploy/compose/docker-compose.yml build` 验证镜像构建成功

## 3. 安全加固

- [x] 3.1 在 `config.Config` 中添加 `DBEncryptionKey string` 字段（env: `HUB_DB_ENCRYPTION_KEY`，32 字节 hex），启动时必填校验
- [x] 3.2 在 `internal/` 下新增 `crypto/` 包：实现 AES-256-GCM 加密/解密函数（`Encrypt`/`Decrypt`）
- [x] 3.3 修改 `internal/handlers/datasources.go`：创建数据源时加密密码，读取时解密，API 响应仅返回 `has_password`
- [x] 3.4 实现内部回调端点的 HMAC-SHA256 签名验证中间件（`internal/middleware/` 或内联在 tasks handler 中）
- [x] 3.5 修改 `services/ai/orchestrator/consumer.py` 和 `tracing.py`：使用 HMAC 签名替代当前明文 secret 头
- [x] 3.6 在 `internal/auth/jwt.go` 中为 JWT 添加 `jti`（UUID v4）声明
- [x] 3.7 实现基于 Redis 的 JWT 吊销列表（`internal/auth/revoke.go`）：`RevokeJWT`/`IsRevoked` 函数，key 自动过期
- [x] 3.8 在认证中间件中集成 JWT 吊销检查
- [x] 3.9 将 `docker-compose.yml` 中所有硬编码密钥改为 `${VAR}` 环境变量引用
- [x] 3.10 修改 `services/ai/rag/knowledge_base.py`：根据 `HUB_ENV` 控制 `allow_reset`（production → False）
- [x] 3.11 在 `internal/handlers/reports.go` 中添加 `runID` UUID 格式校验，拒绝非法路径参数
- [x] 3.12 修复 `cmd/api/main.go` 中 `zap.NewProduction()` 错误被静默丢弃的问题（改为 `log.Fatalf`）
- [x] 3.13 更新 `.env.example`：添加 `HUB_INTERNAL_HMAC_SECRET`、`HUB_DB_ENCRYPTION_KEY`、`HUB_REPORTS_DIR` 说明

## 4. 核心功能补齐

- [x] 4.1 在 `internal/handlers/` 中新增 `/internal/nl2sql` 端点：接收 user_message + schema_json + trace_id，调用 Python gRPC GenerateSQL → Go sqlrun 执行 → 返回结果
- [x] 4.2 修改 `services/ai/orchestrator/graph.py` 的 `nl2sql_node`：从 Mock 数据替换为 HTTP POST 调用 Go `/internal/nl2sql`（带 HMAC 签名）
- [x] 4.3 修改 `services/ai/orchestrator/graph.py` 的 `route_next`：同时支持中英文关键词（"分析"/"analyze"、"报告"/"report"），并检查显式 workflow 参数
- [x] 4.4 实现知识文档的 NATS 发布：修改 `internal/handlers/knowledge.go`，在 `UploadKnowledgeDoc` 中向 NATS `hub.tasks.knowledge_index` 发布消息
- [x] 4.5 新增 `services/ai/orchestrator/knowledge_consumer.py`：订阅 NATS `hub.tasks.knowledge_index`，调用 RAG 模块进行文档分块和向量化，完成后回调 Go API
- [x] 4.6 在 `services/ai/orchestrator/knowledge_consumer.py` 中实现 NATS 消息 `ack`/`nak` 确认机制
- [x] 4.7 修改 `internal/handlers/knowledge.go`：添加 `PATCH /internal/knowledge-docs/{id}/status` 内部回调端点（HMAC 认证），供 Python Worker 更新索引状态
- [x] 4.8 在消息请求体中添加 `workflow` 参数支持（`simple`/`agent_pipeline`/`auto`），修改 `PostMessage` handler 的路由逻辑
- [x] 4.9 修改 `internal/middleware/middleware.go` 的 SSE 端点认证：同时支持 Bearer Header 和 `?token=` 查询参数
- [x] 4.10 实现短期 SSE Token 签发（有效期 1 小时，最小权限），在 `internal/auth/jwt.go` 中添加 `SignSSEToken` 函数

## 5. SSE 浏览器端支持

- [x] 5.1 修改 `web/src/api.ts`：添加 `getSSEUrl(sessionId, sseToken)` 函数，返回带 token 参数的 EventSource URL
- [x] 5.2 修改 `web/src/App.tsx`：用 EventSource 替换 5 秒轮询，处理 `result`、`agent_step`、`approval_required`、`error` 事件
- [x] 5.3 在 EventSource 错误处理中实现断线重连（3 秒延迟自动重连）

## 6. 可观测性提升

- [x] 6.1 修改 `internal/otelsetup/otel.go`：根据 `HUB_OTEL_EXPORTER_ENDPOINT` 环境变量条件初始化 OTLP gRPC exporter
- [x] 6.2 在 `internal/worker/nl2sql.go` 的 gRPC 调用中注入 W3C traceparent metadata
- [x] 6.3 修改 `internal/telemetry/metrics.go`：新增 `hub_nl2sql_duration_seconds` 直方图和 `hub_agent_pipeline_tasks_total` 计数器
- [x] 6.4 在 `services/ai/hub_ai/__main__.py` 的 Python gRPC server 中提取并记录 trace_id 到日志

## 7. 限流器优化

- [x] 7.1 将 `internal/ratelimit/limiter.go` 从固定窗口改为滑动窗口（Sliding Window Log），使用 Redis Sorted Set
- [x] 7.2 保留 Redis 不可用时 fail-open 行为

## 8. 测试基础设施

- [x] 8.1 为 `internal/auth/jwt.go` 编写单元测试：`TestSignAndParse`、`TestExpiredToken`、`TestInvalidSignature`
- [x] 8.2 为 `internal/sqlrun/run.go` 编写单元测试：`TestIsReadOnlySQL_Select`、`TestIsReadOnlySQL_Insert`、`TestIsReadOnlySQL_Drop`
- [x] 8.3 为 `internal/config/config.go` 编写单元测试：`TestLoad_MissingSecret`、`TestLoad_ValidConfig`
- [x] 8.4 为 `internal/schema/cache.go` 和 `internal/schema/discovery.go` 编写集成测试（需要 Postgres/Redis）
- [x] 8.5 为 `services/ai/agents/data_analysis_agent.py` 编写 pytest 测试：`test_empty_dataframe`、`test_basic_stats`
- [x] 8.6 为 `services/ai/rag/knowledge_base.py` 编写 pytest 测试：`test_chunk_document`、`test_chunk_short_document`
- [x] 8.7 在 `Makefile` 中添加 `test-py` 目标（`cd services/ai && pytest`）
- [x] 8.8 更新 `docs/SMOKE_CHECKLIST.md`：添加知识索引和 SSE 推送的冒烟测试步骤

## 9. 前端管理页面

- [x] 9.1 创建 `web/src/pages/DataSourcesPage.tsx`：数据源列表（GET）+ 新建表单（POST）+ 测试连接按钮
- [x] 9.2 创建 `web/src/pages/KnowledgePage.tsx`：知识文档列表（GET）+ 上传表单（POST）
- [x] 9.3 在 `web/src/main.tsx` 中添加路由：`/data-sources` → DataSourcesPage、`/knowledge` → KnowledgePage（使用 React.lazy 懒加载）
- [x] 9.4 在 `web/src/App.tsx` 中添加导航入口（侧边栏或顶栏链接到数据源管理和知识库页面）
- [x] 9.5 在消息输入区添加"深度分析"开关（控制 `workflow` 参数发送）

## 10. 最终验证

- [x] 10.1 运行 `go test ./...` 确保全部 Go 测试通过
- [x] 10.2 运行 `cd services/ai && pytest` 确保全部 Python 测试通过
- [ ] 10.3 运行 `docker compose -f deploy/compose/docker-compose.yml --env-file .env up -d --build` 验证全栈启动
- [ ] 10.4 执行 `docs/SMOKE_CHECKLIST.md` 全部冒烟测试步骤
- [ ] 10.5 验证知识索引全链路：上传文档 → 等待索引完成 → 确认 status 变为 completed
- [ ] 10.6 验证 SSE 实时推送：发送消息 → 浏览器 EventSource 接收事件 → 结果渲染
- [ ] 10.7 验证 Multi-Agent 编排：发送"analyze"关键词消息 → LangGraph 执行 → 报告生成
