## Context

前端项目（`web/`）为 React 18 + TypeScript + Vite SPA，当前已具备：
- Docker 多阶段构建（`Dockerfile.web`，Node 构建 + nginx 运行）
- ErrorBoundary 错误边界（`src/components/ErrorBoundary.tsx`）
- Skeleton 骨架屏加载态（`src/components/Skeleton.tsx`）
- 查询结果以 `ResultTable` 组件展示

唯一缺失的是图表可视化。`recharts` 是 React 生态最常用的图表库（基于 D3），零配置即可使用，与项目现有的轻量级技术栈匹配。

## Goals / Non-Goals

**Goals:**
- 安装 recharts 并集成到消息展示流程
- 新增 `ChartView` 组件，支持 BarChart 和 LineChart
- 用户在表格视图和图表面板间一键切换

**Non-Goals:**
- 不引入图表配置面板（标题编辑、颜色选择等）
- 不添加数据导出功能（Excel 下载已由后端处理）
- 不修改 Dockerfile 或 nginx 配置
- 不重构 ErrorBoundary 或 Skeleton 现有实现

## Decisions

| 决策 | 选择 | 理由 |
|------|------|------|
| 图表库 | `recharts` | React 原生组件、TypeScript 支持好、零依赖（已内置 D3 子集）、社区活跃 |
| X 轴取值 | 第一个非数值列 | 自动推断，无需用户手动配置。典型场景：`SELECT city, COUNT(*) FROM t GROUP BY city` → city 为 X 轴 |
| 数值列判定 | 检查单元格类型是否为 `number` | `apiFetch` 返回的 JSON 已自动解析数值类型，无需额外 schema 查询 |
| 组件位置 | `src/components/ChartView.tsx` | 与其他组件一致 |
| 集成方式 | App.tsx 中 `MessageBody` 内部判断 | 最小改动，直接复用现有 `rows` 数据 |

## Risks / Trade-offs

- **大结果集性能**：前端渲染 10,000+ 数据点的图表可能卡顿 → 取前 100 行渲染图表，超限显示"仅展示前 100 条数据的图表"
- **recharts 包体积**：约 150KB gzipped → 构建时 tree-shaking，Vite 按需打包

## Migration Plan

1. `npm install recharts`
2. 创建 `ChartView.tsx`
3. 修改 `App.tsx` 消息展示逻辑，集成 ChartView
4. 验证 `npm run build` 通过
