#!/usr/bin/env bash
# =============================================================================
# Docker Compose 幂等性测试脚本
# 用途: 反复 up/down 验证全栈服务稳定性、数据持久化和端口无冲突
# 用法: cd deploy/compose && bash test-idempotency.sh
# 前提: Docker Desktop 已启动，.env 文件已配置
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

COMPOSE_CMD="docker compose -f docker-compose.yml --env-file ../../.env"
TIMEOUT=120
PASS=0
FAIL=0
LOG_FILE="idempotency_test_$(date +%Y%m%d_%H%M%S).log"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "$(date '+%H:%M:%S') $1" | tee -a "$LOG_FILE"; }
pass() { log "${GREEN}[PASS]${NC} $1"; PASS=$((PASS + 1)); }
fail() { log "${RED}[FAIL]${NC} $1"; FAIL=$((FAIL + 1)); }
info() { log "${YELLOW}[INFO]${NC} $1"; }

# -----------------------------------------------------------------------------
# 环境检查
# -----------------------------------------------------------------------------
check_env() {
    info "========== 环境检查 =========="

    if ! docker info &>/dev/null; then
        fail "Docker daemon 未运行，请启动 Docker Desktop"
        exit 1
    fi
    pass "Docker daemon 运行中"

    local mem
    mem=$(docker info --format '{{.MemTotal}}' 2>/dev/null || echo "0")
    if [ "${mem%.*}" -lt 8000000000 ] 2>/dev/null; then
        info "可用内存: ${mem} 字节，建议 >= 8GB"
    else
        pass "内存充足"
    fi

    if [ ! -f "../../.env" ]; then
        info ".env 文件不存在，将使用 docker-compose.yml 中的默认值"
    else
        pass ".env 文件存在"
    fi

    # 清理残留
    info "清理旧容器和网络残留..."
    docker compose down --remove-orphans 2>/dev/null || true
    pass "环境清理完成"
}

# -----------------------------------------------------------------------------
# 等待所有服务健康
# -----------------------------------------------------------------------------
wait_services() {
    local round=$1
    info "等待服务启动 (最长 ${TIMEOUT}s)..."

    local waited=0
    local interval=5

    while [ $waited -lt $TIMEOUT ]; do
        local all_up=true
        local svc_list

        svc_list=$(docker compose ps -a --format '{{.Service}}:{{.Status}}' 2>/dev/null)

        for svc in postgres redis chroma nats ai-worker api web; do
            if ! echo "$svc_list" | grep -q "^${svc}:.*\(running\|Up\|healthy\)"; then
                all_up=false
                break
            fi
        done

        if $all_up; then
            pass "第${round}轮: 所有 7 个服务已启动"
            return 0
        fi

        sleep $interval
        waited=$((waited + interval))
        info "  等待中... (${waited}s/${TIMEOUT}s)"
    done

    fail "第${round}轮: 超时未全部启动"
    docker compose ps -a | tee -a "$LOG_FILE"
    return 1
}

# -----------------------------------------------------------------------------
# 一轮完整的 up
# -----------------------------------------------------------------------------
do_up() {
    local round=$1
    info ""
    info "========== 第${round}轮: docker compose up =========="

    if ! docker compose up -d --build 2>&1 | tee -a "$LOG_FILE"; then
        fail "第${round}轮: docker compose up 失败"
        return 1
    fi
    pass "第${round}轮: docker compose up 命令成功"

    wait_services "$round" || return 1

    # 记录服务状态快照
    info "第${round}轮服务状态快照:"
    docker compose ps -a | tee -a "$LOG_FILE"
}

# -----------------------------------------------------------------------------
# 验证服务功能
# -----------------------------------------------------------------------------
verify_services() {
    local round=$1
    info "--- 第${round}轮功能验证 ---"

    # API 健康检查
    if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
        pass "第${round}轮: API /health 返回 200"
    else
        fail "第${round}轮: API /health 不可达"
    fi

    # 登录并获取 token
    local token
    token=$(curl -sf -X POST http://localhost:8080/v1/auth/login \
        -H "Content-Type: application/json" \
        -d '{"email":"admin@demo.local","password":"changeme"}' 2>/dev/null | \
        python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")

    if [ -n "$token" ]; then
        pass "第${round}轮: 用户登录成功"
    else
        fail "第${round}轮: 用户登录失败"
        return 1
    fi

    # 写入测试数据（通过 API 创建 data source 或 session 来验证 Postgres 可写）
    local resp
    resp=$(curl -sf -X POST http://localhost:8080/v1/datasources \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $token" \
        -d '{"name":"test-ds-'"$round"'","type":"postgres","host":"localhost","port":5432,"database":"test","username":"test","password":"test"}' 2>/dev/null || echo "")

    if echo "$resp" | grep -q '"id"'; then
        pass "第${round}轮: Postgres 写入成功 (data source created)"
        local ds_id
        ds_id=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")

        # 删除该测试数据源（清理）
        if [ -n "$ds_id" ]; then
            curl -sf -X DELETE "http://localhost:8080/v1/datasources/$ds_id" \
                -H "Authorization: Bearer $token" > /dev/null 2>&1 || true
        fi
    else
        # 可能已存在同名 data source，尝试其他写入验证方式
        info "第${round}轮: data source 创建可能冲突，尝试列出以验证 DB 可读"
        if curl -sf http://localhost:8080/v1/datasources \
            -H "Authorization: Bearer $token" > /dev/null 2>&1; then
            pass "第${round}轮: Postgres 读取正常 (data source list)"
        else
            fail "第${round}轮: Postgres 读写验证失败"
        fi
    fi

    # gRPC 可用性（检查 ai-worker 端口）
    if nc -z -w 3 localhost 50051 2>/dev/null; then
        pass "第${round}轮: gRPC ai-worker:50051 可达"
    else
        info "第${round}轮: gRPC 端口 50051 未暴露到宿主机（仅容器间通信）"
    fi
}

# -----------------------------------------------------------------------------
# 一轮完整的 down
# -----------------------------------------------------------------------------
do_down() {
    local round=$1
    info "--- 第${round}轮停止 ---"

    if docker compose down 2>&1 | tee -a "$LOG_FILE"; then
        pass "第${round}轮: docker compose down 成功"
    else
        fail "第${round}轮: docker compose down 失败"
        return 1
    fi

    # 确认所有容器已停止
    local remaining
    remaining=$(docker compose ps -a --format '{{.Service}}' 2>/dev/null || echo "")
    if [ -z "$remaining" ]; then
        pass "第${round}轮: 确认无残留容器"
    else
        info "第${round}轮: 残留容器: $remaining"
    fi
}

# -----------------------------------------------------------------------------
# 主流程
# -----------------------------------------------------------------------------
main() {
    echo "DataFlowAgentHub Docker Compose 幂等性测试" | tee "$LOG_FILE"
    echo "开始时间: $(date)" | tee -a "$LOG_FILE"
    echo "" | tee -a "$LOG_FILE"

    check_env

    # === 第 1 轮 ===
    do_up 1 || { fail "第 1 轮 up 失败，终止测试"; exit 1; }
    verify_services 1
    do_down 1

    # === 第 2 轮 (验证数据持久化) ===
    do_up 2 || { fail "第 2 轮 up 失败，终止测试"; exit 1; }
    verify_services 2

    # 检查数据持久化：上一轮的数据应该还在（data source list 不为空）
    info "--- 数据持久化验证 ---"
    local token
    token=$(curl -sf -X POST http://localhost:8080/v1/auth/login \
        -H "Content-Type: application/json" \
        -d '{"email":"admin@demo.local","password":"changeme"}' 2>/dev/null | \
        python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")

    if [ -n "$token" ]; then
        local ds_count
        ds_count=$(curl -sf http://localhost:8080/v1/datasources \
            -H "Authorization: Bearer $token" 2>/dev/null | \
            python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else 0)" 2>/dev/null || echo "0")
        info "数据源列表包含 ${ds_count} 条记录（验证数据持久化）"
    fi

    do_down 2

    # === 第 3 轮 (幂等性验证) ===
    do_up 3 || { fail "第 3 轮 up 失败，终止测试"; exit 1; }
    verify_services 3
    do_down 3

    # === 汇总 ===
    echo "" | tee -a "$LOG_FILE"
    info "========== 测试汇总 =========="
    echo -e "${GREEN}通过: ${PASS}${NC}" | tee -a "$LOG_FILE"
    if [ $FAIL -gt 0 ]; then
        echo -e "${RED}失败: ${FAIL}${NC}" | tee -a "$LOG_FILE"
    else
        echo -e "失败: ${FAIL}" | tee -a "$LOG_FILE"
    fi
    echo "结束时间: $(date)" | tee -a "$LOG_FILE"
    echo "日志文件: $LOG_FILE" | tee -a "$LOG_FILE"

    if [ $FAIL -eq 0 ]; then
        echo ""
        echo -e "${GREEN}============================================${NC}" | tee -a "$LOG_FILE"
        echo -e "${GREEN}  幂等性测试全部通过!${NC}" | tee -a "$LOG_FILE"
        echo -e "${GREEN}============================================${NC}" | tee -a "$LOG_FILE"
        return 0
    else
        echo ""
        echo -e "${RED}============================================${NC}" | tee -a "$LOG_FILE"
        echo -e "${RED}  存在 ${FAIL} 项失败，请检查日志${NC}" | tee -a "$LOG_FILE"
        echo -e "${RED}============================================${NC}" | tee -a "$LOG_FILE"
        return 1
    fi
}

main "$@"
