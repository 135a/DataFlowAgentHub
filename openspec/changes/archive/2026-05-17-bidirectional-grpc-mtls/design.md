## Context

Go API 和 Python Worker 之间当前存在两种通信方式：

- **Go → Python**: gRPC (insecure)，用于 `GenerateSQL`、`RunAgentPipeline`、`Health`
- **Python → Go**: HTTP + HMAC-SHA256，用于 4 个回调端点

这种双协议模式增加维护成本，HMAC 重复代码已在 `shared.py` 中提取但仍需维护。本次设计将全部内部通信统一为 gRPC + mTLS。

## Goals / Non-Goals

**Goals:**
- 统一全部服务间通信为 gRPC 协议
- 用 mTLS 替换 HMAC 和 insecure，建立双向证书验证
- 删除所有 HMAC 相关代码
- 不影响前端用户认证（JWT 不变）
- 不影响异步消息路径（NATS 不变）

**Non-Goals:**
- 不引入 Service Mesh（Istio/Linkerd）
- 不改变 Go API 的 HTTP 前端接口
- 不涉及用户认证体系改动
- 不替换 NATS 消息队列

## Decisions

### 1. 证书体系：自签 CA + 双端证书

**选择：** 内部 CA 签发服务端和客户端证书，有效期 10 年（3650 天）。

**理由：**
- 服务不对外暴露，无需公共 CA
- 10 年有效期覆盖项目生命周期，消除续期运维
- 双端证书（mTLS）确保连接双方身份都可验证
- 相比 HMAC，私钥文件不过网，安全水位更高

### 2. 证书管理：文件 + gitignore + volume 挂载

**选择：** 证书生成脚本 `scripts/gen-certs.sh`，产物存 `certs/`（已 `.gitignore`），Docker volume 运行时挂载。

**理由：**
- 不将证书打包进镜像，防止镜像泄露导致密钥泄露
- `.gitignore` 防止误提交
- 生成脚本可重复执行，方便重新签发

### 3. Proto 组织：现有 proto 文件扩展

**选择：** 在现有 `nl2sql.proto` 中新增 `HubInternalService`，不新建 proto 文件。

**理由：**
- 两个 service 属于同一通信域（Go↔Python）
- 共享同一包名和生成目录，减少文件数量
- `make gen-go` / `make gen-py` 一次编译全部

### 4. Go gRPC 服务端端口：:9090

**选择：** 独立端口，不尝试与 HTTP 端口复用。

**理由：**
- 保持 HTTP 和 gRPC 服务端解耦，各自独立启动和关闭
- chi 路由器与 gRPC 服务端复用端口需要额外 multiplexer，小微项目不值得
- 多一个端口运维成本极低

### 5. Python gRPC 客户端：grpc.aio

**选择：** 异步 consumer 用 `grpc.aio` 做 async/await 调用；同步 tracing 用同步 stub。

**理由：**
- `consumer.py` 和 `knowledge_consumer.py` 已经是 asyncio，`grpc.aio` 原生支持
- `tracing.py` 从同步 LangGraph node 调用，同步 stub 最直接
- 复用一个 gRPC channel 和连接池

### 6. Handler 重构：抽离 Core Logic + Thin 双适配器

**选择：** 将 4 个回调的核心逻辑从 HTTP handler 中抽离为公共方法，HTTP handler 和 gRPC handler 都调用同一方法。

**理由：**
- 避免代码重复
- 迁移期间 HTTP 和 gRPC 可以并存
- 验证通过后删除 HTTP handler 和路由

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| 证书私钥泄露 | `.gitignore` 防护；禁止证书进入镜像；私钥文件 600 权限 |
| 证书过期导致服务中断 | 设 10 年有效期，基本消除 |
| gRPC insecure→mTLS 切换时连接中断 | 先部署证书 + 启用 mTLS 监听，再切换客户端连接 |
| tracing.py 同步调 gRPC 性能 | gRPC channel 复用连接池，开销低于原有 HTTP 调用 |
| 混合协议过渡期混乱 | 先加 gRPC 新路径，验证通过后再删 HTTP 旧路径 |
