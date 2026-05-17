## 1. Docker Compose 幂等性测试（2h）

- [x] 1.1 准备测试环境：确认 Docker 资源充足（内存≥8GB）、清理旧容器和网络残留
- [x] 1.2 首轮启动：执行 `docker compose up -d --build`，记录 `docker compose ps -a` 输出和每个服务的健康状态
- [x] 1.3 首轮验证：调用 API `/health` 端点、创建测试用户并写入数据、验证 Postgres/Redis 可读写
- [x] 1.4 首轮停止：执行 `docker compose down`（不带 `-v`），确认所有容器已停止且无残留进程
- [x] 1.5 第二轮启动：再次 `docker compose up -d --build`，对比服务状态与首轮是否一致
- [x] 1.6 第二轮验证：确认上一轮写入的数据仍然存在、API 健康检查通过、gRPC 连接正常
- [x] 1.7 第三轮循环：重复 down/up，验证幂等性——三轮输出一致、无端口冲突、无数据污染
- [x] 1.8 整理测试日志，记录异常项和修复结果

## 2. FIXME/TODO 清理验证（2h）

- [x] 2.1 全仓库扫描：使用 `rg -i "(TODO|FIXME|HACK|XXX)"` 扫描全部代码文件（Go/Python/TS/配置），确认输出为空
- [x] 2.2 编译阻断级核验：逐条确认 `docs/ISSUES.md` #1-#3（InternalHMACSecret、ssebus 导入、未使用 import）已修复，执行 `go build ./...` 通过
- [x] 2.3 运行时阻断级核验：逐条确认 `docs/ISSUES.md` #4-#8（Dockerfile.ai 拷贝范围、proto 桩代码、knowledge.go NATS 发布、consumer.py headers 变量、迁移文件）修复状态
- [x] 2.4 更新 `docs/ISSUES.md`：将已修复问题标注为"已修复"，未修复的记录当前状态和处理建议

## 3. 编写 Agent 设计文档（3h）

- [x] 3.1 编写概述章节：Agent 在 DataFlowAgentHub 中的定位、设计目标、整体架构图
- [x] 3.2 编写 LangGraph 图设计章节：`StateGraph` 完整拓扑（NL2SQL → 分支 → Analysis → Report）、节点定义、条件边逻辑、AgentState 数据流
- [x] 3.3 编写 Prompt 工程章节：NL2SQL/Analysis/Report 每个节点的 System Prompt 模板、设计原则、约束策略
- [x] 3.4 编写 Prompt 迭代日志：记录 Prompt 版本演进、每次修改的原因和效果评估
- [x] 3.5 编写状态管理章节：`AgentState` 字段完整说明、MemorySaver 机制、Checkpoint 保存/恢复流程、进程重启限制
- [x] 3.6 编写关键决策记录：Mock→真实实现路线、MemorySaver vs PostgresSaver 选择、Python gRPC servicer 当前状态
- [x] 3.7 文档收尾：添加版本标注（日期 + commit hash）、目录导航、后续改进方向指引
