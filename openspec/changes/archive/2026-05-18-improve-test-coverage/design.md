## Context

当前项目代码审计显示测试覆盖存在两个缺口：(1) 前端 22 个 TypeScript/React 文件零测试；(2) Python AI 核心组件（chart_agent、report_generation_agent、LangGraph graph、NATS consumer）缺少单元测试。此外前端大量使用内联样式，影响可维护性。

MVP 功能已基本稳定，现在补测试和重构样式风险可控，且能提升后续开发的回归安全性。

## Goals / Non-Goals

**Goals:**
- 搭建前端测试基础设施（Vitest + React Testing Library），覆盖关键组件
- 将内联样式迁移为 CSS Modules，优先处理复用度高的组件
- 为 Python 侧 chart_agent、report_generation_agent、graph、consumer 编写单元测试
- 完善 Python 测试基础设施（fixtures、mock helpers）

**Non-Goals:**
- 不追求 100% 测试覆盖率（聚焦核心路径）
- 不重写现有功能逻辑
- 不引入 CSS-in-JS 方案（CSS Modules 增量迁移）
- 不修改 Go 侧代码

## Decisions

### 1. 前端测试框架：Vitest + React Testing Library

| 方案 | 优点 | 缺点 |
|------|------|------|
| **Vitest** (选) | 与 Vite 共享配置，零额外构建，运行快，jest 兼容 API | 相对较新 |
| Jest | 生态成熟 | 需要单独配置，与 Vite 有兼容问题 |

**理由**：项目使用 Vite 构建，Vitest 是自然选择。配置成本最低，API 与 Jest 兼容，迁移无痛。

### 2. 前端测试文件组织：colocated `*.test.tsx`

测试文件放在对应组件同一目录下（如 `ChatPanel.test.tsx` 与 `ChatPanel.tsx` 同级），而非集中到 `__tests__/`。理由：组件和测试一同维护，重构时不会遗漏。

### 3. Python 测试 Mock 策略

| 组件 | Mock 策略 |
|------|-----------|
| chart_agent | Mock `matplotlib.pyplot`，验证绘图函数调用次数和参数 |
| report_generation_agent | Mock 文件写入，验证 Markdown/Excel 内容结构 |
| graph | Mock LLM 调用和 gRPC 回调，验证状态机节点路由 |
| consumer | Mock NATS 消息和 gRPC 客户端，验证消息处理循环 |

不 mock 底层数据结构操作（pandas DataFrame、matplotlib 计算），只 mock 有副作用的调用（文件 I/O、网络、LLM）。

### 4. CSS Modules 迁移策略

按"高复用度优先"原则分批迁移：
- 第一批：ChatPanel、SessionSidebar（核心 UI）
- 第二批：ProgressPanel、ResultTable、ChartView
- 第三批：DataManagementPanel、ModeSelector

接收 props 的样式通过 `classnames` 库组合 className，不通过内联 style 计算。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|----------|
| Python 测试中 mock matplotlib 渲染可能遗漏跨平台字体问题 | 集成测试（Docker 内运行）覆盖字体渲染 |
| CSS Modules 迁移可能引入视觉回归 | 逐个组件迁移，提交粒度小，便于回滚 |
| 前端测试 mock 过多导致测试价值降低 | 关键路径（消息渲染、SSE 重连、角色路由）写集成测试 |
| Python graph 测试中 mock gRPC 调用复杂 | 封装 gRPC 客户端接口，用 `unittest.mock.patch` 替换 |
