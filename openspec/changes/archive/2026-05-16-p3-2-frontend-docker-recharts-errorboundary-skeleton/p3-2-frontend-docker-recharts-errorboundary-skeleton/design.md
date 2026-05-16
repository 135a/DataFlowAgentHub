## Context

`docker-compose.yml` 中 web 服务已定义，引用 `Dockerfile.web`。前端使用 Vite + React + TypeScript 构建，nginx 提供静态文件服务并反向代理 API。

当前组件状态：
- `ChartView.tsx`：Recharts 柱状图/折线图，自动检测数值列
- `ErrorBoundary.tsx`：类组件错误边界，开发模式显示调用栈，支持重试
- `Skeleton.tsx`：脉冲动画骨架屏（line/rect 变体 + PageSkeleton）

## Goals / Non-Goals

**Goals:**
- 创建 `Dockerfile.web` 多阶段构建
- 确认 `docker compose up` 时 web 服务正常启动
- 前端三大组件（Recharts/ErrorBoundary/Skeleton）随镜像一起交付

**Non-Goals:**
- 不新增 UI 组件库
- 不修改 nginx.conf

## Decisions

### 1. 多阶段 Docker 构建

**选择**：Stage 1 `node:22-alpine` 执行 `npm ci && npm run build`，Stage 2 `nginx:alpine` 复制 `dist/` 和 `nginx.conf`。

```dockerfile
# Stage 1: Build
FROM node:22-alpine AS build
WORKDIR /app
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Serve
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

### 2. nginx 配置复用

使用现有 `web/nginx.conf`，无需修改。已验证：API 代理到 `http://api:8080`，SPA fallback 到 `index.html`，静态资源 1 年缓存。

## Risks / Trade-offs

- 首次构建需下载 npm 依赖 → 利用 Docker 层缓存加速后续构建
