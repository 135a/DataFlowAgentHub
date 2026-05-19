## ADDED Requirements

### Requirement: Go 代码 CI 流水线

系统 SHALL 在每次 push 和 PR 时自动执行 Go 代码的编译、检查和测试。

#### Scenario: Push 触发 CI
- **WHEN** 代码被 push 到 main 分支或创建 PR
- **THEN** GitHub Actions 自动运行
- **AND** 执行 `go mod tidy` 检查
- **AND** 执行 `go build ./...`
- **AND** 执行 `go vet ./...`
- **AND** 执行 `go test ./...`

#### Scenario: 测试失败告警
- **WHEN** CI 流水线中任一阶段失败
- **THEN** GitHub Actions 将运行状态标记为失败
- **AND** PR 页面显示检查未通过

### Requirement: Docker 镜像构建

系统 SHALL 在 CI 中自动构建 Docker 镜像并验证 Dockerfile 正确性。

#### Scenario: Docker 构建验证
- **WHEN** CI 运行
- **THEN** 执行 `docker build` 构建 API 镜像
- **AND** 执行 `docker build` 构建 ai-worker 镜像
- **AND** 构建成功后删除临时镜像（不推送至仓库）

### Requirement: Python 依赖锁定

系统 SHALL 生成 `requirements.lock` 文件锁定所有传递依赖版本，确保可复现构建。

#### Scenario: 生成 lockfile
- **WHEN** 执行 `pip-compile services/ai/requirements.txt`
- **THEN** 生成 `services/ai/requirements.lock` 文件
- **AND** lockfile 纳入版本管理

#### Scenario: CI 中使用 lockfile
- **WHEN** CI 安装 Python 依赖
- **THEN** 使用 `pip install -r services/ai/requirements.lock`
- **AND** 确保每次安装的依赖版本一致
