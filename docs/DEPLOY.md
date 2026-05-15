# 单机部署（Docker Compose，无 Kubernetes）

## 前提

- Linux 服务器或本机已安装 **Docker** 与 **Compose v2**。
- 域名与 TLS（生产）：在宿主机使用 **Nginx** 或 **Caddy** 反向代理到 `api:8080`，证书用 Let’s Encrypt 或自有证书。

## 步骤

1. 克隆仓库，在仓库根目录复制环境变量模板：  
   `cp .env.example .env`  
   编辑 `.env`，**至少设置** `HUB_JWT_SECRET`（长随机串）。可选设置 `OPENAI_API_KEY` 以启用真实 NL2SQL。
2. 启动：  
   `docker compose -f deploy/compose/docker-compose.yml --env-file .env up -d --build`
3. 健康检查：`curl -s http://127.0.0.1:8080/health`
4. 登录获取 JWT：  
   `curl -s -X POST http://127.0.0.1:8080/v1/auth/login -H "Content-Type: application/json" -d "{\"email\":\"admin@demo.local\",\"password\":\"changeme\"}"`

## 日志与备份

- **日志**：`docker compose ... logs -f api ai-worker`。
- **备份**：对 Postgres 卷做定期快照；`pg_dump` 导出 `hub` 库。

## 密钥

不要把 `.env` 提交到 Git；生产环境优先使用密钥管理（Docker secrets、云厂商 SM 等）。
