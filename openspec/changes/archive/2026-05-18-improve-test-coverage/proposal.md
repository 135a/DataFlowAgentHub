## Why

项目代码质量审计显示两个薄弱环节：前端（React/TypeScript）零测试覆盖，Python AI 核心组件（chart_agent、report_generation_agent、LangGraph graph 构建、NATS consumer）测试覆盖率低。这两个问题影响回归安全性和重构信心，需要在 MVP 功能稳定后补上。

## What Changes

- **前端测试框架搭建**：引入 Vitest + React Testing Library，为关键组件编写单元测试和集成测试
- **前端样式重构**：将内联样式迁移到 CSS Modules，提高可维护性
- **Python 测试补充**：为 chart_agent、report_generation_agent、LangGraph graph 构建、NATS consumer 编写单元测试
- **Python 测试基础设施完善**：补充 pytest fixtures 和 mock 工具函数

## Capabilities

### New Capabilities
- `frontend-tests`: 前端测试框架与组件测试覆盖
- `python-test-coverage`: Python AI 核心组件测试覆盖

### Modified Capabilities
<!-- 无现有 spec 需要修改 -->

## Impact

| 领域 | 影响 |
|------|------|
| web/ | 新增 `vitest.config.ts`，组件文件旁加 `*.test.tsx`，CSS Modules 文件 |
| services/ai/tests/ | 新增 chart_agent、report_generation_agent、graph、consumer 测试文件 |
| services/ai/ | 可能需小幅重构以便于 mock（如依赖注入） |
| package.json | 新增 vitest、@testing-library/react 等 devDependencies |
| CI | 测试命令可并行运行 Go + Python + 前端测试 |
