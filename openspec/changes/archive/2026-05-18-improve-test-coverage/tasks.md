## 1. 前端测试基础设施搭建

- [x] 1.1 安装 Vitest、@testing-library/react、@testing-library/jest-dom、jsdom 等 devDependencies
- [x] 1.2 创建 web/vitest.config.ts，配置与 Vite 共享构建，使用 jsdom 环境
- [x] 1.3 在 package.json 中添加 test/test:run 脚本
- [x] 1.4 创建 web/src/test-setup.ts，配置 @testing-library/jest-dom 全局匹配器
- [x] 1.5 验证空测试通过，确认 CI 可执行 `npx vitest run`

## 2. 前端组件测试

- [x] 2.1 编写 ChatPanel 组件测试：渲染用户消息、助理回复、错误状态、SQL 结果表、报告内容
- [x] 2.2 编写 useSSE hook 测试：断线重连、事件类型分发、组件卸载清理
- [x] 2.3 编写 SessionSidebar 组件测试：会话列表渲染、高亮当前会话、新建会话交互、加载骨架屏
- [x] 2.4 编写 LoginPage 组件测试：预填演示凭据、登录失败错误展示
- [x] 2.5 编写 ProgressPanel 组件测试：步骤进度渲染、估算剩余时间显示
- [x] 2.6 编写 ResultTable 组件测试：动态列合并、空数据渲染、大数据截断

## 3. 前端 CSS Modules 迁移

- [x] 3.1 为 ChatPanel 创建 ChatPanel.module.css，迁移内联样式，使用 classnames 处理条件样式
- [x] 3.2 为 SessionSidebar 创建 SessionSidebar.module.css，迁移内联样式
- [x] 3.3 为 ProgressPanel 创建 ProgressPanel.module.css，迁移内联样式
- [x] 3.4 为 ResultTable 创建 ResultTable.module.css，迁移内联样式
- [x] 3.5 为 ChartView 创建 ChartView.module.css，迁移内联样式

## 4. Python 测试基础设施完善

- [x] 4.1 在 services/ai/tests/conftest.py 中添加标准 DataFrame fixture（数值列、字符串列、混合类型、空数据）
- [x] 4.2 添加 mock grpc internal_client fixture，mock task_callback/run_step_callback/internal_nl2sql
- [x] 4.3 添加 mock matplotlib fixture，避免测试中实际渲染图表
- [x] 4.4 添加 mock NATS message fixture，模拟有效/无效 payload

## 5. Python Chart Agent 测试

- [x] 5.1 测试柱状图选择逻辑（分类数据列）
- [x] 5.2 测试折线图选择逻辑（时间关键词列名）
- [x] 5.3 测试饼图选择逻辑（比例数据）
- [x] 5.4 测试大数据集截断（超过 50 行的采样行为）
- [x] 5.5 测试空数据集处理
- [x] 5.6 测试 matplotlib 异常处理

## 6. Python Report Generation Agent 测试

- [x] 6.1 测试 Markdown 生成（包含分析摘要、请求内容、图表引用）
- [x] 6.2 测试 Excel 文件生成（包含数据 sheet）
- [x] 6.3 测试空数据处理

## 7. Python LangGraph Graph 测试

- [x] 7.1 测试图节点结构（4 个节点 + 入口路由）
- [x] 7.2 测试 route_next：成功时路由到 analysis，失败时路由到 __end__
- [x] 7.3 测试 route_after_analysis：有数值数据时路由到 chart，无则路由到 report
- [x] 7.4 测试 route_after_chart：路由到 report
- [x] 7.5 测试中英文关键词路由（"analyze"/"分析"等）

## 8. Python NATS Consumer 测试

- [x] 8.1 测试有效 agent pipeline 消息处理
- [x] 8.2 测试无效 payload 错误处理
- [x] 8.3 测试超时机制（120 秒超时取消与失败报告）
