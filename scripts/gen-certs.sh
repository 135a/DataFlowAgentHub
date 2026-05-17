#!/bin/bash
# 生成内部 mTLS 证书
# 用法: bash scripts/gen-certs.sh [certs-dir]
# 默认输出到项目根目录的 certs/ 文件夹

# 防止 Git Bash 将 /CN 等参数转换为 Windows 路径
export MSYS2_ARG_CONV_EXCL="*"

set -euo pipefail

CERTS_DIR="${1:-$(dirname "$0")/../certs}"
CERTS_DIR="$(cd "$(dirname "$CERTS_DIR")" && pwd)/$(basename "$CERTS_DIR")"
DAYS=3650
CA_SUBJ="/CN=DataFlowAgentHub Internal CA"

echo "=== 生成内部 mTLS 证书 ==="
echo "输出目录: $CERTS_DIR"
echo "有效期: $DAYS 天"
echo ""

mkdir -p "$CERTS_DIR"
cd "$CERTS_DIR"

# 1. CA 证书
echo "[1/5] 生成 CA 私钥和证书..."
openssl genrsa -out ca.key 2048
openssl req -x509 -new -key ca.key -out ca.crt -days "$DAYS" \
  -subj "$CA_SUBJ" -nodes

# 2. Go 服务端证书 (用于 Go gRPC server :9090)
echo "[2/5] 生成 Go 服务端证书..."
openssl genrsa -out go-server.key 2048
openssl req -new -key go-server.key -out go-server.csr \
  -subj "/CN=api" -nodes
printf "subjectAltName=DNS:api,DNS:localhost,IP:127.0.0.1" > san-go-server.cfg
openssl x509 -req -in go-server.csr -CA ca.crt -CAkey ca.key \
  -out go-server.crt -days "$DAYS" -CAcreateserial \
  -extfile san-go-server.cfg
rm san-go-server.cfg

# 3. Go 客户端证书 (用于 Go 连接 Python Worker)
echo "[3/5] 生成 Go 客户端证书..."
openssl genrsa -out go-client.key 2048
openssl req -new -key go-client.key -out go-client.csr \
  -subj "/CN=go-client" -nodes
openssl x509 -req -in go-client.csr -CA ca.crt -CAkey ca.key \
  -out go-client.crt -days "$DAYS" -CAcreateserial

# 4. Python 服务端证书 (用于 Python gRPC server :50051)
echo "[4/5] 生成 Python 服务端证书..."
openssl genrsa -out py-server.key 2048
openssl req -new -key py-server.key -out py-server.csr \
  -subj "/CN=ai-worker" -nodes
printf "subjectAltName=DNS:ai-worker,DNS:localhost,IP:127.0.0.1" > san-py-server.cfg
openssl x509 -req -in py-server.csr -CA ca.crt -CAkey ca.key \
  -out py-server.crt -days "$DAYS" -CAcreateserial \
  -extfile san-py-server.cfg
rm san-py-server.cfg

# 5. Python 客户端证书 (用于 Python 连接 Go)
echo "[5/5] 生成 Python 客户端证书..."
openssl genrsa -out py-client.key 2048
openssl req -new -key py-client.key -out py-client.csr \
  -subj "/CN=py-client" -nodes
openssl x509 -req -in py-client.csr -CA ca.crt -CAkey ca.key \
  -out py-client.crt -days "$DAYS" -CAcreateserial

# 清理临时文件
rm -f go-server.csr go-client.csr py-server.csr py-client.csr ca.srl

# 设置权限
chmod 600 *.key

echo ""
echo "=== 生成完成 ==="
ls -lh "$CERTS_DIR"
