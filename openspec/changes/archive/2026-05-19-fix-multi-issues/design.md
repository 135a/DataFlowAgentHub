## Context

项目在 MySQL 迁移后有 6 个结构性问题待修复，涵盖构建可用性、代码可维护性、自动化、生产部署、供应商耦合和依赖管理。这些问题互不阻塞，可并行实施。

## Goals / Non-Goals

**Goals:**
- 让项目能在当前 Go 工具链下编译
- 将 handler.go 拆分为可维护的模块
- 建立 GitHub Actions CI 流水线
- 提供 K8s 部署所需的清单文件
- 解耦 LLM 供应商，修复 PG prompt bug
- 锁定 Python 依赖版本

**Non-Goals:**
- 不引入额外的第三方依赖（Go 或 Python）
- 不改动业务逻辑或 API 接口
- 不涉及实际部署到 K8s 集群（仅提供清单）
- 不实现多供应商动态路由（仅做接口抽象）

## Decisions

### 1. Go 版本：降级到 1.23（而非 1.24）

**决策：** 将 `go.mod` 改为 `go 1.23`。

**理由：** 经测试，代码库使用的标准库特性在 Go 1.23 中均受支持，且 1.23 是当前广泛可用的稳定版本。

**替代方案：** `go 1.24` — 也是可选项，但对开发环境要求更高。

### 2. handler.go 拆分策略

**决策：** 按功能域拆分为 ~5 个文件，保持 `package handlers` 不变。

```
handlers/
├── handlers.go      # App struct, Routes(), 公共辅助函数 (缩减至 ~200 行)
├── auth.go          # 已有，不变
├── datasources.go   # 已有，不变
├── knowledge.go     # 已有，不变
├── datasets.go      # 已有，不变
├── tables.go        # 已有，不变
├── reports.go       # 已有，不变
├── data.go          # 已有，不变
├── tasks.go         # 已有，不变
├── messages.go      # PostMessage 相关逻辑 (从 handlers.go 拆分)
└── admin.go         # 管理端点 (从 handlers.go 拆分)
```

**理由：** `handlers.go` 中的 `PostMessage` 逻辑（~300 行）和管理端点可以自然拆分为独立文件，沿用项目已有的 handler 文件组织模式。

### 3. LLM Provider 抽象设计

**决策：** 在 Python 侧新增 `llm_provider.py`，定义协议类 + 工厂函数。

```python
class LLMProvider(ABC):
    @abstractmethod
    async def generate_sql(self, request) -> tuple[str, str]: ...
    @abstractmethod
    async def ask(self, question: str, context: str) -> str: ...

class OpenAIProvider(LLMProvider):
    def __init__(self):
        self.client = AsyncOpenAI(...)     # 现有逻辑迁移至此

class FallbackProvider(LLMProvider):
    async def generate_sql(self, request): # 返回 "SELECT 1 AS ok"

def create_provider() -> LLMProvider:       # 工厂函数
    if os.environ.get("OPENAI_API_KEY"):
        return OpenAIProvider()
    return FallbackProvider()
```

**理由：** 协议类定义清晰，工厂函数集中控制实例化逻辑，后续添加 Anthropic 或本地模型只需新增实现类。

### 4. CI/CD 选择 GitHub Actions

**决策：** 使用 GitHub Actions，配置在 `.github/workflows/ci.yml`。

**理由：** 项目托管在 GitHub，Actions 零额外成本、配置简单、与 PR 深度集成。

### 5. K8s 清单结构

**决策：** 基础清单（Deployment + Service + ConfigMap），不引入 Helm。

**理由：** 当前阶段不需要 Helm 的模板化能力，纯 YAML 更易理解和修改。后续可按需升级。

### 6. Python 锁文件

**决策：** 使用 `pip-compile`（来自 `pip-tools`）生成 `requirements.lock`。

**理由：** 零配置、与现有 `requirements.txt` 兼容、社区标准方案。

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| Go 1.23 可能缺少某些已使用的特性 | 已验证代码库，仅使用标准库基础功能，无泛型或新版本特有特性依赖 |
| handler 拆分可能引入回归 | 拆分不改逻辑，仅移动代码；`go vet` + 测试可验证 |
| LLM 接口抽象后 RAG 路径需同步改造 | RAG 路径（`_rag_answer`）一并纳入抽象范围 |
| K8s 清单未经实际部署验证 | 清单仅供参考，标注 "production-ready review required" |
