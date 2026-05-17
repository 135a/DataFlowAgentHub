## Why

项目接近 MVP 交付阶段，需要完成三项收尾工作：验证 Docker Compose 全栈部署的幂等性和稳定性、确保代码中无遗留的 FIXME/TODO 标记、以及编写面试核心文档 `AGENT_DESIGN.md`。这三项工作直接关系到项目的可演示性和技术传播能力。

## What Changes

- **全栈 Docker Compose 幂等性测试**：反复执行 `docker compose up -d --build` 和 `docker compose down`，验证所有服务（postgres、redis、chroma、nats、ai-worker、api、web）能稳定启动和停止，无端口冲突、数据卷污染或竞态条件
- **FIXME/TODO 清理验证**：确认所有代码文件（Go、Python、TypeScript）中无 FIXME/TODO/HACK/XXX 等遗留标记；针对 `docs/ISSUES.md` 中记录的遗留问题逐条核验是否已修复
- **Agent 设计文档**：编写 `docs/AGENT_DESIGN.md`，涵盖 LangGraph 图设计（节点编排、分支逻辑）、Prompt 迭代演进策略、状态管理方案（MemorySaver、Checkpoint、序列化）、NL2SQL→Analysis→Report 完整链路说明

## Capabilities

### New Capabilities
- `docker-compose-testing`: 全栈 Docker Compose 反复 up/down 的幂等性测试验证
- `code-cleanup-verification`: 代码 FIXME/TODO 清理验证，确保项目无遗留标记
- `agent-design-doc`: 编写 Agent 设计文档，覆盖 LangGraph 设计、Prompt 迭代、状态管理

### Modified Capabilities
<!-- 本次变更不修改现有能力规范 -->

## Impact

- 受影响文件：`deploy/compose/docker-compose.yml`（测试目标）、`docs/AGENT_DESIGN.md`（新建）、`docs/ISSUES.md`（核验参考）
- 无 API 变更，无破坏性改动
- 测试工作需 Docker 环境支持，预计 2 小时内完成
