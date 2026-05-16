# frontend-docker-production 规格说明

## ADDED Requirements

### Requirement: Docker 镜像构建

系统 SHALL 提供 `Dockerfile.web`，使用多阶段构建生成包含前端静态资源和 nginx 的 Docker 镜像。

#### Scenario: 构建成功

- **WHEN** 执行 `docker build -f Dockerfile.web -t hub-web .`
- **THEN** 镜像构建成功，大小 < 50MB（nginx:alpine 基础镜像 + 静态资源）

#### Scenario: docker compose 集成

- **WHEN** 执行 `docker compose -f deploy/compose/docker-compose.yml up -d --build`
- **THEN** web 服务正常启动，访问 `http://localhost:80` 显示前端页面

### Requirement: Recharts 图表渲染

系统 SHALL 在 SQL 查询结果包含数值列时，提供 Recharts 图表视图切换能力。

#### Scenario: 柱状图切换

- **WHEN** SQL 结果包含数值列且用户点击"图表"按钮
- **THEN** 显示 Recharts 柱状图，X 轴为文本列，Y 轴为数值列

### Requirement: ErrorBoundary 错误边界

系统 SHALL 在组件渲染失败时显示错误边界，提供错误信息和重试按钮。

#### Scenario: 渲染异常捕获

- **WHEN** 子组件抛出渲染异常
- **THEN** ErrorBoundary 显示错误消息、调用栈（开发模式）和重试按钮

### Requirement: Skeleton 骨架屏

系统 SHALL 在数据加载期间显示 Skeleton 骨架屏占位符。

#### Scenario: 页面加载骨架屏

- **WHEN** 页面或组件数据正在异步加载
- **THEN** 显示脉冲动画的骨架屏占位符，数据加载完成后替换为实际内容
