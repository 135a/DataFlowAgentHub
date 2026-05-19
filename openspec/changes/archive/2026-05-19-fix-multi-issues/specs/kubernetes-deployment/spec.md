## ADDED Requirements

### Requirement: K8s Deployment 清单

系统 SHALL 提供 Kubernetes Deployment 配置，支持 Go API 和 Python ai-worker 的生产部署。

#### Scenario: API Deployment
- **WHEN** 执行 `kubectl apply -f deploy/k8s/`
- **THEN** Go API 服务以 Deployment 形式部署
- **AND** 暴露 Service（ClusterIP，端口 8080）
- **AND** 包含存活和就绪探针（`/health` 端点）

#### Scenario: ai-worker Deployment
- **WHEN** 执行 `kubectl apply -f deploy/k8s/`
- **THEN** Python ai-worker 以 Deployment 形式部署
- **AND** 暴露 gRPC Service（ClusterIP，端口 50051）
- **AND** 包含存活探针（gRPC health check）

### Requirement: ConfigMap 配置管理

系统 SHALL 使用 ConfigMap 管理非敏感配置项，使用 Secret 管理敏感配置。

#### Scenario: ConfigMap 包含环境变量
- **WHEN** Pod 启动
- **THEN** ConfigMap 中的配置项以环境变量形式注入容器
- **AND** 敏感项（JWT_SECRET、API_KEY 等）使用 Secret 而非 ConfigMap

### Requirement: 水平扩缩容支持

系统 SHALL 配置合理的资源请求与限制，支持 HPA（Horizontal Pod Autoscaler）。

#### Scenario: 资源限制已配置
- **WHEN** 检查 K8s Deployment 清单
- **THEN** 每个容器包含 `requests` 和 `limits` 资源定义
- **AND** ai-worker 的 CPU 和内存配置高于 API 服务
