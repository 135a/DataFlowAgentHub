# DataFlowAgentHub 项目问题总结

> 生成日期: 2026-05-15 | 基于 commit `0cbb6ac`

---

## 一、编译阻断级 Bug（代码无法通过编译）

| # | 文件 | 问题 |
|---|------|------|
| 1 | `internal/config/config.go` | 缺少 `InternalHMACSecret` 字段，`tasks.go:55,123` 引用了 `a.Cfg.InternalHMACSecret` 但 Config 结构体中不存在 |
| 2 | `internal/handlers/tasks.go` | 未导入 `ssebus` 包，第 113、165 行使用了 `a.Bus.Publish()` 但编译会失败 |
| 3 | `internal/handlers/knowledge.go` | 第 7 行导入了 `"io"` 但未使用，Go 编译器会拒绝 |

**修复建议**:
- 在 `config.Config` 中添加 `InternalHMACSecret string` 字段，env tag 为 `HUB_INTERNAL_HMAC_SECRET`
- 在 `tasks.go` 的 import 块中补上 `"github.com/dataflowagenthub/hub/internal/ssebus"`
- 删除 `knowledge.go` 中未使用的 `"io"` import

---

## 二、运行时阻断级 Bug（编译通过但运行时报错）

| # | 文件 | 问题 |
|---|------|------|
| 4 | `Dockerfile.ai` | **仅拷贝了 `services/ai/hub_ai/` 目录**，未拷贝 `orchestrator/`、`agents/`、`rag/` 目录。`__main__.py` 第 100 行 `import orchestrator.graph` 会在容器中抛出 `ModuleNotFoundError` |
| 5 | `api/proto/nl2sql/v1/nl2sql.proto` | `RunAgentPipeline` RPC 在 proto 中已定义，但生成的 Go 客户端桩代码（`internal/gen/`）中不包含该方法，Go gRPC 客户端无法调用此 RPC |
| 6 | `internal/handlers/knowledge.go:111` | `TODO: Publish task to NATS` — 知识文档上传后只写入了数据库，从未发布到 NATS，Python 消费者永远收不到索引任务，文档永远停留在 `pending` 状态 |
| 7 | `services/ai/orchestrator/consumer.py:37-77` | `process_message` 函数的 `headers` 变量在 try 块内定义，若异常发生在 headers 赋值之前（第 37 行），except 块会因 `NameError` 再次崩溃 |
| 8 | `internal/migrate/` (002-004) | ~~三个补充迁移文件原在 `migrations/`(005-007)，现已统一到 `internal/migrate/`(002-004)，由 Go embed 机制自动执行~~ **已修复** |

---

## 三、安全漏洞

| # | 类别 | 问题 |
|---|------|------|
| 9 | 明文密码 | `data_sources` 表的 `password TEXT NOT NULL` 列以明文存储数据库密码，一旦 DB 泄露则全部外部数据源凭据暴露 |
| 10 | 硬编码密钥 | `docker-compose.yml` 中存在多处开发密钥：`HUB_JWT_SECRET: dev-insecure-change-me`、`HUB_INTERNAL_HMAC_SECRET: dev-hmac-secret-change-me`、`HUB_SEED_PASSWORD: changeme` |
| 11 | 无 TLS | gRPC 使用 `insecure.NewCredentials()`，HTTP 为明文，NATS/Redis/ChromaDB 均无 TLS，所有内部通信可被中间人监听 |
| 12 | 路径遍历 | `internal/handlers/reports.go:33` 将 URL 中的 `runID` 直接拼入 `filepath.Join`，虽然 UUID 格式限制了风险，但缺乏显式校验 |
| 13 | JWT 无法吊销 | `internal/auth/jwt.go` 中的 JWT 不含 `jti`（JWT ID）声明，无黑名单机制，签发的令牌在过期前无法主动吊销 |
| 14 | ChromaDB 安全 | `services/ai/rag/knowledge_base.py` 使用 `Settings(allow_reset=True)`，允许任何人通过 Chroma API 重置 Collection |

---

## 四、功能缺失与未完成

### 4.1 核心功能

| # | 模块 | 问题 |
|---|------|------|
| 15 | LangGraph 编排器 | `services/ai/orchestrator/graph.py:17-34` — `nl2sql_node` 是 **Mock 实现**，返回硬编码的假数据 `[{"mock_col": 1, "value": 100}]`，Multi-Agent 流程在未接入真实 NL2SQL 前不产生实际价值 |
| 16 | 知识库索引 | `internal/handlers/knowledge.go:111` NATS 发布未实现（同 #6），整个 RAG 索引流程断裂 |
| 17 | gRPC RunAgentPipeline | Go 端 `internal/worker/nl2sql.go` 未暴露 `RunAgentPipeline` RPC 的客户端封装（即使 proto 修复后也需要添加） |
| 18 | SSE 前端不可用 | `web/src/App.tsx:87-89` 注释明确指出：浏览器 `EventSource` API 不支持自定义请求头，而 SSE 端点需要 Bearer 认证，导致 SSE 实时推送功能在浏览器端完全无法使用 |

### 4.2 测试

| # | 问题 |
|---|------|
| 19 | **Go 代码零测试**：整个 `internal/` 目录无任何 `_test.go` 文件 |
| 20 | **Python 代码零测试**：`services/ai/` 目录无任何 `test_*.py` 文件 |
| 21 | `make test` 等同于 `go test ./...`，不会运行任何实际测试 |

### 4.3 运维与可靠性

| # | 模块 | 问题 |
|---|------|------|
| 22 | SSE 总线 | `internal/ssebus/bus.go` — 纯内存实现，进程重启后所有事件丢失；通道缓冲区仅 32，慢消费者会被静默丢弃 |
| 23 | 数据库迁移 | `internal/migrate/migrate.go` — 无迁移版本追踪，每次启动重跑所有 SQL，依赖 `IF NOT EXISTS` 保证幂等性，但无法处理 `ALTER TABLE ADD COLUMN` 等操作 |
| 24 | NATS 消费 | `services/ai/orchestrator/consumer.py` — 无消息确认（ack/nak），消费者崩溃时消息丢失；daemon 线程无重启逻辑；混用 asyncio 和 threading |
| 25 | OTel 遥测 | `internal/otelsetup/otel.go` — 注释标注 "MVP：无 exporter"，实际不导出任何链路数据 |
| 26 | LangGraph 状态 | `orchestrator/graph.py` 使用 `MemorySaver`，重启后所有正在运行的 Agent 工作流状态丢失 |
| 27 | 限流器 | `internal/ratelimit/limiter.go` — 固定窗口算法存在边界突发问题；Redis 不可用时完全放行（fail-open） |

### 4.4 前端功能缺口

| # | 问题 |
|---|------|
| 28 | 无数据源管理界面（无法在 UI 中添加/配置数据库连接） |
| 29 | 无知识库/文档管理界面（无法上传或查看 RAG 文档） |
| 30 | 无 SSE 实时推送（#18）— 当前只能通过 5 秒轮询获取任务状态 |
| 31 | Token 存储在 `localStorage`，存在 XSS 泄露风险 |

### 4.5 其他限制

| # | 模块 | 问题 |
|---|------|------|
| 32 | Schema 发现 | `internal/schema/discovery.go:42` — 仅查询 `public` schema，无法发现其他 schema 中的表 |
| 33 | 数据源连接 | `internal/handlers/datasources.go:69` — 仅支持 `"postgres"` 类型，硬编码，无法接入 MySQL 等其他数据库 |
| 34 | 语言国际化 | 多处中文关键词硬编码用于路由判断（`"分析"`、`"报告"`、`"export"`），非中文用户无法触发 Multi-Agent 和审批流程 |
| 35 | 报表下载 | `internal/handlers/reports.go` — 路径 `/tmp/reports/` 硬编码，无可配置性 |
| 36 | 知识文档 | 不支持文件上传（仅 JSON body），无文件大小限制检查 |
| 37 | `pkg/` 目录 | 完全为空（仅有 `.gitkeep`），Go 共享包未建立 |
| 38 | Python 依赖 | `report_generation_agent.py` 使用了 `tabulate` 库（`df.to_markdown()`），但 `requirements.txt` 中未声明 |
| 39 | Dockerfile 版本 | `Dockerfile.api` 使用 Go 1.22，但 `go.mod` 指定 Go 1.25.0，版本不匹配 |

---

## 五、OpenSpec 变更待完成项

| # | 变更 | 待完成 |
|---|------|--------|
| 40 | `2026-05-15-add-schema-discovery` | 任务 7.1（本地 demo_sales 端到端验证）和 7.2（2+ 表冒烟测试）标记为未完成 |
| 41 | `openspec/specs/` | 主线规范目录为空（仅 README 占位），所有规范仍以 delta 形式存在于归档变更中 |

---

## 六、问题优先级矩阵

```
                    影响大
                      |
         ┌────────────┼────────────┐
         │  #1 #2 #3  │  #4 #5 #6  │
         │  (编译)    │  #8 #15    │
         │            │  #18       │
 修复简单├────────────┼────────────┤修复复杂
         │  #10 #14   │  #11 #13   │
         │  #19 #20   │  #9 #22    │
         │  #38 #39   │  #23 #24   │
         └────────────┼────────────┘
                      |
                    影响小
```

**建议修复顺序**:

1. **立即修复**（阻断开发/部署）: #1, #2, #3, #4, #5, #6, #8
2. **高优先级**（安全/功能断裂）: #9, #10, #12, #14, #15, #18, #7
3. **中优先级**（可靠性/完成度）: #22, #23, #24, #26, #19, #20
4. **低优先级**（增强/扩展）: #11, #13, #25, #28-#37
