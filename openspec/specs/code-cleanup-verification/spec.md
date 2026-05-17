# Code Cleanup Verification

## Purpose

确保代码仓库中零 FIXME/TODO/HACK/XXX 遗留标记，并对 ISSUES.md 中记录的已知 Bug 逐条核验修复状态。

## Requirements

### Requirement: 代码中零 FIXME/TODO 遗留

系统 SHALL 在整个代码仓库（Go、Python、TypeScript、配置文件）中不存在 FIXME、TODO、HACK、XXX 等临时标记。

#### Scenario: 全仓库扫描零结果

- **WHEN** 执行 `rg -i "(TODO|FIXME|HACK|XXX)" --type-add 'code:*.{go,py,ts,tsx,js,sql,yml,yaml,toml,json}'` 扫描全部代码文件
- **THEN** 输出为空（无匹配结果）

#### Scenario: 排除文档中的计划性引用

- **WHEN** 扫描命中 `docs/` 目录下的规划文档（如 ISSUES.md、UPGRADE_GUIDE.md）
- **THEN** 此类引用不视为待修复项，但需确认所引用的代码位置已无对应标记

### Requirement: ISSUES.md 记录的问题逐条核验

系统 SHALL 针对 `docs/ISSUES.md` 中记录的编译阻断级和运行时阻断级 Bug（#1-#8），逐条确认修复状态并更新文档。

#### Scenario: 编译阻断级 Bug 全部确认已修复

- **WHEN** 逐条核验 #1（InternalHMACSecret 字段）、#2（ssebus 导入）、#3（未使用 import）
- **THEN** 代码编译通过，`go build ./...` 零错误

#### Scenario: 运行时阻断级 Bug 确认修复或记录状态

- **WHEN** 逐条核验 #4-#8
- **THEN** 已修复的标记为"已修复"，未修复的在 ISSUES.md 中更新状态并标注负责人
