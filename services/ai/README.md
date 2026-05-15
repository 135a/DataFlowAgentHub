# hub-ai（Python NL2SQL worker）

- 构建时由 `Dockerfile.ai` 生成 `gen/nl2sql/v1/` 的 gRPC 桩代码。
- 本地开发：在仓库根执行 `make gen-py`（需安装 `grpcio-tools`），再：

```bash
cd services/ai
set PYTHONPATH=gen
python -m hub_ai
```
