## ADDED Requirements

### Requirement: 前端 Docker 镜像构建

系统 SHALL 提供 Dockerfile 用于构建前端生产镜像，镜像基于 nginx:alpine 托管 Vite 构建产物。

#### Scenario: 构建前端镜像

- **WHEN** 执行 `docker build -f Dockerfile.web --build-arg VITE_API_BASE_URL=/api -t hub-web .`
- **THEN** 成功生成包含 nginx 配置和静态文件的镜像，大小不超过 50MB

#### Scenario: 构建时注入 API 地址

- **WHEN** 构建镜像时设置 `VITE_API_BASE_URL=https://example.com/api`
- **THEN** 前端所有 API 请求使用 `https://example.com/api` 为前缀

### Requirement: nginx SPA 路由支持

nginx 配置 SHALL 支持 SPA 路由，所有非静态资源路径回退到 `index.html`，API 请求转发到 Go 后端。

#### Scenario: SPA 路由回退

- **WHEN** 浏览器直接访问 `/data-sources` 路径
- **THEN** nginx 返回 `index.html`（而非 404），由 React Router 接管渲染

#### Scenario: API 请求反向代理

- **WHEN** 前端发起请求到 `/v1/sessions`
- **THEN** nginx 将请求代理转发到 `http://api:8080/v1/sessions`

#### Scenario: 静态资源缓存

- **WHEN** 浏览器请求 `/assets/index-xxxxx.js`（带 hash 的静态资源）
- **THEN** nginx 返回 `Cache-Control: max-age=31536000` 头

### Requirement: docker-compose 集成

前端服务 SHALL 作为 `web` 服务集成到 `deploy/compose/docker-compose.yml` 中，与其他服务并行启动。

#### Scenario: 一键启动全栈

- **WHEN** 执行 `docker compose -f deploy/compose/docker-compose.yml up -d`
- **THEN** `web` 服务在 80 端口可用，通过浏览器访问可看到 DataFlowAgentHub 页面

#### Scenario: 前端依赖 API 服务

- **WHEN** `api` 服务未就绪时访问前端
- **THEN** 页面正常渲染（静态资源加载成功），API 调用返回 502，页面显示错误状态而非白屏
