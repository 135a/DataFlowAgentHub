## Why

当前前端缺少 Docker 化部署方案，无法在 docker compose 全栈中启动前端服务，演示只能靠本地 `npm run dev`。同时前端代码质量偏低（全 `any` 类型、无错误边界、无加载骨架、硬编码常量），达不到线上演示的体验标准。需要在一次迭代中同时补齐部署能力和 UI 体验，使项目具备一键线上演示的完整能力。

## What Changes

- **Docker 化前端**：新增 `Dockerfile.web`，使用 nginx 托管 Vite 构建产物，通过 `VITE_API_BASE_URL` 环境变量在构建时注入 API 地址
- **docker-compose 集成**：新增 `web` 服务，与 api、ai-worker 等并行启动，暴露 80 端口
- **ErrorBoundary**：新增全局错误边界组件，防止单点异常导致白屏
- **TypeScript 类型化**：定义 API 响应类型（Session、Message、DataSource、KnowledgeDoc、ApprovalTask），消除所有 `any`
- **加载骨架**：新增 Skeleton 组件，替代纯文字 "loading..."
- **硬编码修复**：KnowledgePage 中 workspace ID 改为从 JWT Claims 解析或通过 props/context 传入
- **SSE 重连改进**：添加指数退避策略（1s → 2s → 4s → 8s → 上限 30s）

## Capabilities

### New Capabilities

- `frontend-docker-deploy`: 前端 Docker 镜像构建与 nginx 托管，支持运行时 API 地址注入
- `frontend-error-handling`: 全局错误边界与局部错误回退
- `frontend-types`: API 响应类型定义，消除 any 类型
- `frontend-ux-polish`: 加载骨架、状态反馈、SSE 重连优化

### Modified Capabilities

（无现有 capabilities 需要修改）

## Impact

- 新增文件：`Dockerfile.web`、`web/nginx.conf`、`web/src/components/ErrorBoundary.tsx`、`web/src/components/Skeleton.tsx`、`web/src/types/api.ts`、`web/src/hooks/useWorkspaceId.ts`
- 修改文件：`web/src/App.tsx`、`web/src/main.tsx`、`web/src/api.ts`、`web/src/pages/KnowledgePage.tsx`、`web/src/pages/DataSourcesPage.tsx`、`web/src/pages/LoginPage.tsx`、`deploy/compose/docker-compose.yml`
- 依赖新增：无新 npm 包，纯 TypeScript + React 内置能力
