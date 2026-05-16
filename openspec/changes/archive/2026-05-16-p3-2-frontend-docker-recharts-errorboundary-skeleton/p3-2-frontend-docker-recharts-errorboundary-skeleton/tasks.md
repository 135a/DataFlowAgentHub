## 1. Docker 镜像构建

- [x] 1.1 创建 `Dockerfile.web` 多阶段构建文件（node:22-alpine 构建 + nginx:alpine 运行）
- [x] 1.2 验证 `docker build -f Dockerfile.web -t hub-web .` 构建成功
- [x] 1.3 验证 `docker compose -f deploy/compose/docker-compose.yml up -d web` 服务正常

## 2. 前端组件确认

- [x] 2.1 确认 ChartView.tsx（Recharts）在 Docker 镜像中正常工作
- [x] 2.2 确认 ErrorBoundary.tsx 错误边界正确捕获异常
- [x] 2.3 确认 Skeleton.tsx 骨架屏动画正常渲染
