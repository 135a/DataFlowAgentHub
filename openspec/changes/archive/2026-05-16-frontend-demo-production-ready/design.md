## Context

当前前端为 React 18 + TypeScript + Vite 单页应用，共 8 个源文件（含 3 个页面组件）。开发时通过 Vite 代理到 Go API，无 Docker 化。代码质量方面：全 `any` 类型、无错误边界、内联 style、硬编码 workspace ID。

目标：补齐 Docker 部署能力和 UI 体验，使其达到线上演示标准。不改动后端代码。

## Goals / Non-Goals

**Goals:**
- 前端可作为 Docker 容器集成到 docker compose 全栈中
- API 地址在构建时通过环境变量注入（`VITE_API_BASE_URL`）
- nginx 配置支持 SPA 路由（所有路径回退到 index.html）
- 全局 ErrorBoundary 防止白屏，提供重试按钮
- 全部 API 响应和组件 props 具备 TypeScript 类型
- 异步操作有加载骨架/过渡反馈
- KnowledgePage 不再硬编码 workspace ID
- SSE 重连使用指数退避

**Non-Goals:**
- 不引入 UI 组件库 (MUI/Ant Design)
- 不引入状态管理库 (Redux/Zustand)
- 不改造布局结构（路由和页面组织不变）
- 不添加主题系统（维持内联 style，后续迭代再做）
- 不添加 i18n

## Decisions

### D1: 前端 Docker 化方案 — nginx 独立镜像

**选择**: 多阶段构建 — Node.js 编译 + nginx:alpine 托管

```
[Vite build] → [nginx:alpine 静态文件] → [Go API :8080]
```

**备选方案**:
- Go embed 方案：将 `dist/` 用 `embed.FS` 打包进 Go 二进制。优点：少一个容器。缺点：Go 二进制增大，前端更新必须重编译 Go，职责混淆。未采用。
- Node serve 方案：`vite preview` 直接暴露。优点：最简单。缺点：不适合生产，无 gzip/缓存等优化。未采用。

**理由**: nginx 是静态文件托管的事实标准，gzip 压缩、缓存头、SPA 路由一应俱全，镜像仅 ~15MB。

### D2: API 地址注入 — 构建时环境变量

**选择**: Vite 构建时通过 `VITE_API_BASE_URL` 注入，nginx 启动后不可更改。

```dockerfile
ARG VITE_API_BASE_URL=/api
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN npm run build
```

**备选方案**: 运行时通过 nginx `sub_filter` 替换占位符。优点：镜像构建一次可部署多环境。缺点：需要 nginx 模块、配置复杂。演示阶段不需要，未采用。

**理由**: 演示环境 API 地址固定，构建时注入最简单可靠。

### D3: nginx SPA 路由 — try_files

**选择**: `try_files $uri $uri/ /index.html` 标准 SPA 回退。

所有非静态资源请求（如 `/data-sources`、`/knowledge`）回退到 `index.html`，由 React Router 处理。API 请求（`/v1/*`、`/health`、`/version`）通过反向代理转发到 Go API。

### D4: Workspace ID 来源 — 从 JWT Claims 解析

**选择**: 新增 `useWorkspaceId` hook，从 `localStorage` 中的 JWT token 解析 workspace_id（jwt payload.workspace_id）。

```typescript
function useWorkspaceId(): string {
  const token = localStorage.getItem("token");
  if (!token) throw new Error("not authenticated");
  const payload = JSON.parse(atob(token.split(".")[1]));
  return payload.workspace_id;
}
```

**理由**: JWT Claims 中已包含 `workspace_id` 字段（Go 后端签发），无需额外 API 调用。比硬编码健壮，比 props drilling 简单。

### D5: 错误边界粒度 — 全局单一边界

**选择**: 一个 `ErrorBoundary` 包裹整个路由树，fallback 显示"出错了"+ 重试按钮。

```
<ErrorBoundary>
  <Routes>
    ...
  </Routes>
</ErrorBoundary>
```

**备选方案**: 每个页面独立 ErrorBoundary。优点：局部错误不影响其他页面。缺点：增加复杂度，当前页面间无强隔离需求。演示阶段全局即可。

### D6: TypeScript 类型 — 集中定义 + 逐步应用

**选择**: `web/src/types/api.ts` 集中定义所有 API 响应类型。现有组件逐步迁移，优先覆盖 API 调用返回值和组件 props。

类型定义优先级：
1. API 响应类型（`Session`、`ApiMessage`、`MessageContent`、`DataSource`、`KnowledgeDoc`、`ApprovalTask`）
2. 组件 props 类型
3. 事件处理函数类型

### D7: 加载骨架 — 轻量自定义组件

**选择**: 自写 `<Skeleton>` 组件（纯 CSS 动画），不引入第三方库。

```
<Skeleton variant="line" width="60%" />
<Skeleton variant="rect" width="100%" height={200} />
```

**理由**: 需求简单（3-4 种变体），额外依赖不划算。CSS `@keyframes` 动画足以实现脉冲效果。

## Risks / Trade-offs

- **[风险] nginx 反向代理增加一跳延迟** → 内网通信，延迟 <1ms，可忽略
- **[风险] VITE_API_BASE_URL 构建时固化** → 演示环境地址固定，不需要运行时切换
- **[风险] JWT 解析在前端暴露 payload** → JWT payload 本身非加密（仅签名），不包含敏感信息（无密码），可接受
- **[风险] TypeScript 类型可能与后端实际响应不一致** → 后端 API 相对稳定，且类型定义基于现有代码阅读，不是凭空猜测
