## Why

当前前端代码已有 Recharts 图表、ErrorBoundary 错误边界、Skeleton 加载骨架屏组件，但缺少 Docker 镜像构建能力。`docker-compose.yml` 中 `web` 服务引用了 `Dockerfile.web`，但该文件尚未创建。需要创建多阶段 Dockerfile（Node 构建 + Nginx 运行），使前端可一键部署。

## What Changes

- 创建 `Dockerfile.web`：多阶段构建（Node 22-alpine 构建 + nginx:alpine 运行）
- 确认 Recharts、ErrorBoundary、Skeleton 组件正常工作
- 确认 nginx 反向代理 API、SPA 路由 fallback 正确

## Capabilities

### New Capabilities

- `frontend-docker-production`: 前端 Docker 镜像构建能力，包含 Recharts 图表、ErrorBoundary 错误边界、Skeleton 骨架屏

### Modified Capabilities

<!-- 无现有 spec 变更 -->

## Impact

- 新增 `Dockerfile.web`
- 依赖 `web/nginx.conf`（已存在）、`web/package.json`（已存在）
