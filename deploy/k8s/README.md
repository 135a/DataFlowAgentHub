# Kubernetes 部署清单

> **注意:** 这些清单是生产部署的基础模板。实际使用前需根据环境进行审查和调整。

## 前提条件

- Kubernetes 集群 (v1.25+)
- 以下基础设施服务已在集群内或可通过网络访问：
  - MySQL 8.x
  - Redis
  - NATS (JetStream)
  - ChromaDB

## 部署步骤

### 1. 创建命名空间（可选）

```bash
kubectl create namespace hub
kubectl config set-context --current --namespace=hub
```

### 2. 创建 Secret（敏感配置）

```bash
kubectl create secret generic hub-secret \
  --from-literal=HUB_JWT_SECRET='your-32-char-secret-here' \
  --from-literal=HUB_DB_ENCRYPTION_KEY='your-hex-64-char-key' \
  --from-literal=HUB_MYSQL_ROOT_PASSWORD='your-mysql-password' \
  --from-literal=HUB_LLM_API_KEY='your-openai-api-key'
```

### 3. 部署 ConfigMap

```bash
kubectl apply -f configmap.yaml
```

### 4. 部署服务

```bash
kubectl apply -f api-deployment.yaml
kubectl apply -f ai-worker-deployment.yaml
```

### 5. 验证部署

```bash
kubectl get pods -l app=hub-api
kubectl get pods -l app=hub-ai-worker
kubectl get svc hub-api
```

## 生产就绪审查清单

- [ ] Secret 使用外部密钥管理（Vault / External Secrets Operator），而非 `kubectl create secret`
- [ ] ConfigMap 中的非敏感默认值已按环境调整
- [ ] HPA 配置已根据实际负载调整 resource requests/limits
- [ ] Ingress 配置已添加（TLS 终止）
- [ ] 持久卷（PV/PVC）已为报告和知识文件目录配置
- [ ] 镜像标签使用具体版本而非 `latest`
- [ ] Pod 安全策略 / 安全上下文已配置
- [ ] NetworkPolicy 已配置以限制服务间通信
- [ ] 基础设施服务（MySQL、Redis、NATS、ChromaDB）的 HA 配置已就绪
