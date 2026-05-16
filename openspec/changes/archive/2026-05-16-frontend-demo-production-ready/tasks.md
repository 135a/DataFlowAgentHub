## 1. 前端 Docker 部署

- [x] 1.1 创建 `Dockerfile.web`：多阶段构建（node:18-alpine + nginx:alpine），接受 `VITE_API_BASE_URL` 构建参数
- [x] 1.2 创建 `web/nginx.conf`：SPA try_files 回退 + `/v1`、`/health`、`/version` 反向代理到 `api:8080` + 静态资源长期缓存
- [x] 1.3 在 `deploy/compose/docker-compose.yml` 新增 `web` 服务：暴露 80 端口，依赖 `api`，注入 `VITE_API_BASE_URL` 构建参数

## 2. TypeScript 类型定义

- [x] 2.1 创建 `web/src/types/api.ts`：定义 `Session`、`ApiMessage`、`MessageContent`（联合类型：TextContent | SqlResultContent | ErrorContent | ReportContent）、`DataSource`、`KnowledgeDoc`、`ApprovalTask`
- [x] 2.2 更新 `web/src/api.ts`：`apiFetch` 加泛型支持，`getSSEUrl` 参数类型化
- [x] 2.3 更新 `web/src/App.tsx`：消除 `any` 类型，`sessions`、`messages`、`runSteps` 使用新类型
- [x] 2.4 更新 `web/src/pages/DataSourcesPage.tsx`：`items` 使用 `DataSource[]`
- [x] 2.5 更新 `web/src/pages/KnowledgePage.tsx`：`items` 使用 `KnowledgeDoc[]`

## 3. 错误处理

- [x] 3.1 创建 `web/src/components/ErrorBoundary.tsx`：捕获子组件渲染错误，显示回退 UI（错误消息 + 重试按钮 + 开发模式堆栈）
- [x] 3.2 在 `web/src/main.tsx` 中包裹 ErrorBoundary 到路由外层
- [x] 3.3 更新 `web/src/api.ts`：在 `apiFetch` 中处理 401（清除 token 跳转登录）、网络错误（统一错误消息）

## 4. UX 优化

- [x] 4.1 创建 `web/src/components/Skeleton.tsx`：支持 `line` 和 `rect` 变体，CSS 脉冲动画
- [x] 4.2 更新 `web/src/main.tsx`：`Suspense fallback` 替换为 Skeleton 页面布局占位
- [x] 4.3 创建 `web/src/hooks/useWorkspaceId.ts`：从 JWT token payload 解析 workspace_id
- [x] 4.4 更新 `web/src/pages/KnowledgePage.tsx`：使用 `useWorkspaceId` hook 替代硬编码 workspace ID
- [x] 4.5 更新 `web/src/App.tsx`：SSE 重连改为指数退避（1s → 2s → 4s → 8s → 16s → 上限 30s）
- [x] 4.6 更新 `web/src/App.tsx`：审批面板空状态显示"暂无待审批项"

## 5. 验证

- [x] 5.1 `npm run build` 在 web 目录下成功执行，无 TypeScript 错误
- [ ] 5.2 `docker compose -f deploy/compose/docker-compose.yml build web` 成功（受阻：Docker Hub 网络不可达）
- [ ] 5.3 确认 docker compose 全栈启动后浏览器可访问前端页面（受阻：依赖 5.2）
