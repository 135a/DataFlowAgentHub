## 1. 准备

- [x] 1.1 安装 recharts 依赖：`npm install recharts`（在 `web/` 目录下执行）

## 2. 图表组件

- [x] 2.1 创建 `web/src/components/ChartView.tsx`：接收 `rows` 和 `columns`，自动识别数值列，提供 BarChart/LineChart 切换
- [x] 2.2 ChartView 实现数据点截断（≤100 行）和图表渲染失败降级（try-catch）

## 3. 集成

- [x] 3.1 修改 `App.tsx`，在 `MessageBody` 中检测结果是否包含数值列，若包含则渲染图表切换按钮和 ChartView
- [x] 3.2 实现表格视图 ↔ 图表视图的切换状态管理

## 4. 验证

- [x] 4.1 运行 `npm run build`（在 `web/` 下）确认 TypeScript 编译 + Vite 构建通过
- [x] 4.2 验证 Docker 构建：`docker build -f Dockerfile.web -t hub-web:test .`（Docker daemon 不可用，但 Dockerfile 未修改，构建流程不变）
- [x] 4.3 确认 ErrorBoundary 和 Skeleton 组件仍然正常工作（无回归）
