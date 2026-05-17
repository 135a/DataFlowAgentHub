## Context

项目已通过冒烟测试（commit `95217d3`），全栈 Docker Compose 可一键启动。当前需要验证反复 up/down 场景下的幂等性和稳定性，确保给面试官演示时不会出现启动失败。同时需确认代码中无遗留 FIXME/TODO 标记，并编写 Agent 设计文档作为面试技术考察的支撑材料。

## Goals / Non-Goals

**Goals:**
- 验证 `docker compose up -d --build` / `docker compose down` 在至少 3 轮循环中所有 7 个服务稳定启动，无端口冲突或数据污染
- 确认整个代码仓库（Go、Python、TypeScript、配置文件）中零 FIXME/TODO/HACK/XXX 遗留标记
- 产出 `docs/AGENT_DESIGN.md`，内容覆盖：LangGraph 图拓扑、Prompt 迭代策略、Checkpoint/状态管理、节点间数据流

**Non-Goals:**
- 不修改 Docker Compose 文件自身的结构
- 不新增测试框架或 CI 集成
- 不实现新的 Agent 功能或修改图拓扑
- 不编写 API 文档或用户手册

## Decisions

### D1: 幂等性验证采用手动脚本 + 对比法

**选择**：编写 shell 脚本循环 `up/down`，每次 up 后执行 `docker compose ps -a` 和健康检查，对比两轮输出是否一致。
**理由**：项目无 CI 环境，手动脚本可复用且透明。对比法能直观发现服务状态差异。
**备选**：使用 `docker compose` 内置健康检查 + `--wait`——但因 ai-worker 和 api 的启动依赖链较长，`--wait` 超时机制不够可靠。

### D2: FIXME/TODO 清理采用 rg 全量扫描

**选择**：使用 `rg -i "(TODO|FIXME|HACK|XXX)" --type-add 'code:*.{go,py,ts,tsx,js,sql,yml,yaml,toml,json}'` 全仓库扫描，零结果为通过。
**理由**：此命令涵盖全部代码和配置，之前已运行确认无残留，本任务实质上是"验证 + 记录"。
**备选**：逐个 grep——效率低且易遗漏文件类型。

### D3: AGENT_DESIGN.md 结构

**选择**：文档分为五章：概述→ LangGraph 图设计 → Prompt 工程 → 状态管理 → 关键决策记录。
**理由**：
- **概述**：交代整体架构位置和设计目标
- **LangGraph 图设计**：节点拓扑、边逻辑、条件分支、序列化数据流——面试官最关心的部分
- **Prompt 工程**：每个节点的 System Prompt 模板及迭代日志，展示工程化思路
- **状态管理**：AgentState 结构定义、MemorySaver/Checkpoint 机制、序列化与恢复
- **关键决策记录**：Mock→真实节点的演进路径、为什么选 MemorySaver 而非 PostgresSaver、Python 侧 gRPC servicer 的当前状态

## Risks / Trade-offs

- **[风险] Docker 宿主机资源不足**：7 个容器（含 ChromaDB 和 NATS）可能耗尽内存 → 在测试前确认至少 8GB 可用内存
- **[风险] 数据卷残留导致状态不一致**：`docker compose down -v` 会清空数据库，但保留数据卷便于下一轮测试；轮次之间需确认 `down` 不带 `-v` 以模拟真实重启场景
- **[权衡] AGENT_DESIGN.md 可能随代码演进过时**：文档末尾添加版本标注和日期，作为快照文档而非动态文档

## Open Questions

- 无待决问题——三项任务均为确定性工作，无需额外调研
