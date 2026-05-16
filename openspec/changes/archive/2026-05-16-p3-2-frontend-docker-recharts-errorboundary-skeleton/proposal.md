## Why

前端当前仅支持表格形式展示 SQL 查询结果，缺乏图表可视化能力。数据分析场景中，用户期望看到趋势图、柱状图等直观图表。此外，前端 Docker 化、ErrorBoundary 错误边界和 Skeleton 骨架屏加载态均已实现，但缺少图表库（recharts）来完善"前端体验合格"的最后一块拼图。

## What Changes

- 安装 `recharts` 图表库及其类型定义
- 在消息展示中新增图表渲染模式：当查询结果包含数值列时，自动提供柱状图/折线图切换
- 新增 `ChartView` 组件，支持 BarChart 和 LineChart 两种模式
- Docker 化、ErrorBoundary、Skeleton 三项已实现，本变更仅做验证确认

## Capabilities

### New Capabilities

- `query-result-chart`: 查询结果图表可视化——当 SQL 查询返回数值数据时，系统 SHALL 支持以柱状图或折线图形式展示结果，用户可在表格视图和图表面板之间切换

### Modified Capabilities

<!-- 无现有规范需要修改 -->

## Impact

- 新依赖：`recharts`（运行时）
- 新增文件：`web/src/components/ChartView.tsx`
- 修改文件：`web/src/App.tsx`（消息展示中集成图表切换）、`web/package.json`（添加依赖）
- 不受影响：Docker 构建流程、nginx 配置、API 层
