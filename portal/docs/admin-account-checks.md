# 管理员账号降智检查

入口：管理员菜单「账号降智检查」，路径 `/admin/account-checks`。

- 仅显示和允许检查 OpenAI 账号；左侧多选列表显示账号 ID、名称及全部绑定分组。搜索框下方固定显示「全选账号」，可选择所有分页中匹配搜索条件的账号，也可取消全选。
- 页面工作区固定在当前窗口内，账号列表和账号卡片分别在盒子内滚动，检查按钮固定在底部。
- 每个已选账号使用一张紧凑卡片，模型、分组、定时检查和该账号的结果一起展示。页面/动画使用 160px 高度的小预览，文字在卡片内滚动，两者均可放大查看。
- 统一题目会分别发送给每个账号；逐账号出题要求每个已选账号都有题目。
- 卡片操作栏的「开始检查」仅检查当前账号，读取统一题目或该账号自己的题目，不要求其他账号填写题目。重新检查仅替换当前账号的结果；卡片检查和批量检查共享最多 3 个并发，同一账号运行或排队时不可重复提交。
- 每个账号默认预填 `gpt-6-astra`，可选择或输入其他模型，留空使用 Portal 默认检查模型 `gpt-6-astra`。
- 已选账号可直接修改绑定分组；只能提交分组 ID，不开放任意 Core 路径透传。
- 每个账号可创建、编辑、启停和删除定时检查计划。计划默认显示最新结果及执行时间，点击「最近 3 条」展开历史；Portal 固定计划最多保留 3 条结果。
- 最多同时执行 3 个账号，其余排队。独立记录本轮的账号、题目、状态、结果；排队项可取消，运行中的账号可通过「终止检查」取消 Portal 发往上游的当前请求，「停止全部」会逐个终止运行项并取消尚未发送的排队项。
- 文字直接显示。完整 HTML / SVG（包括 Markdown 代码块）自动在沙箱中预览，支持内联动画、重新播放、全屏和原文查看。外部资源不可加载。
- 手动检查记录保存在 Portal 数据库，按管理员隔离，每个账号仅保留最近 3 次。自动恢复账号卡片、题目、模型、最新结果和时间，可在卡片内切换历史记录。
- 当前选中的账号单独保存在 Portal，移除卡片、取消勾选和取消全选都会先保存选择，刷新后不再自动加回。重新选择仍可查看原历史。首次升级尚无选择记录时，仅恢复 Core 中仍存在的历史账号。
- 恢复卡片先读取全部分页的当前 Core OpenAI 账号；Core 已删除的账号不会因历史恢复，名称和分组使用当前数据。Core 查询失败会提示重试，不会当作账号删除，也不会清除保存的历史或选择。
- 已接收的检查会在普通浏览器刷新或离开后继续执行并保存；页面每 5 秒恢复正在执行的记录。只有管理员明确点击「终止检查」或「停止全部」才会取消服务端到上游的请求，并将历史记录保存为「已终止」。超过 6 分钟的中断记录会显示失败并允许重试。尚未发送的浏览器排队项不属于已保存记录。
- 上游明确返回 502/503/504 且尚无输出时，在原记录内最多自动重试一次（间隔 2 秒，总计仍受 5 分钟限制），记录实际尝试次数。1008 策略关闭、鉴权失败和已有部分输出均不自动重放。断流显示中文解释，未完成的 HTML/SVG 折叠为原始输出，不执行预览；原始错误保留在技术详情及历史中。

## API 与边界

Portal 新增以下管理员专用 API，均校验当前用户为 active admin：

- `GET /api/v1/admin/account-checks/accounts?page=1&search=...`：通过 Core 官方账号列表 API 查询 `platform=openai`，仅返回展示字段。
- `GET /api/v1/admin/account-checks/accounts/{accountID}/models`：确认平台后读取 Core 模型列表。
- `PUT /api/v1/admin/account-checks/accounts/{accountID}/groups`：仅更新目标账号的 `group_ids`。
- `POST /api/v1/admin/account-checks/accounts/{accountID}/test`：接收 `prompt`、`model_id`，确认平台后通过 Core 官方 API 读取该账号连接配置，由 Portal 直接向其上游发送题目。返回 SSE 心跳及一条包含 `text`、`html`、`model`、可选 `error` 的结果。
- `DELETE /api/v1/admin/account-checks/accounts/{accountID}/test`：仅终止当前管理员对该账号最新的运行中手动检查，取消 Portal 到上游的请求并保存「已终止」状态；重复调用不会影响已完成记录或后续新检查。
- `GET|POST /api/v1/admin/account-checks/accounts/{accountID}/scheduled-test-plans`：列出或创建定时检查计划。
- `PUT|DELETE /api/v1/admin/account-checks/accounts/{accountID}/scheduled-test-plans/{planID}`：修改或删除属于该账号的计划。
- `GET /api/v1/admin/account-checks/accounts/{accountID}/scheduled-test-plans/{planID}/results?limit=1|3`：读取最新结果或最近 3 条记录。

检查调用上限 5 分钟，Portal 每 10 秒发送心跳，并延长此请求的写入期限。结果保留模型文案；中断或失败时保留部分文字，不执行未完成的页面。只做人工检查展示，不自动宣称账号降智。

手动检查的自定义题目由 Portal 发送给指定账号的上游；定时检查沿用 Core 已有计划执行器和默认探测题目。

### Core 自定义题目兼容性（2026-10-05）

官方 Core `0.2.13` 的管理员测试接口虽然接收 `prompt`，但 OpenAI Responses 分支调用 `createOpenAITestPayload` 时未传入题目，函数内固定发送 `hi`。因此 Portal 的统一题目和逐账号题目都能正确显示、保存，却收到问候语；检查显示成功仅表示请求成功完成。

手动检查改由 Portal 服务端使用当前管理员 JWT，通过 Core 官方单账号导出 API 读取该账号的连接配置，再直接向该账号的上游发送题目。支持 API Key 的 Responses / Chat Completions 和普通 OAuth 的 Responses；遵循模型映射及已配置代理。凭据只在服务端内存中使用，不回传浏览器、不写入历史或日志。Core 的权限和二次验证拒绝直接导致检查失败，不退回服务器管理员密钥或 `hi` 探测。

只部署 Portal API 即可生效，不修改 Core 代码、镜像、数据库或账号分组。定时连通性检查仍沿用 Core 原接口。特殊账号或不可用的连接配置明确报错，禁止悄悄换账号或退回默认问候语。

2026-10-05 18:55（Asia/Shanghai）已部署 Portal API 修复镜像 `portal-portal-api:account-prompt-20261005`，镜像 ID `sha256:112c2d30fee25a432682a94d2982096d3b733d4033efa3bf6ff2500ce0f08d59`。回滚镜像为 `portal-portal-api:rollback-account-prompt-20261005`，备份在 `/opt/backups/portal-account-prompt-20261005`。Core 保持 `weishaw/sub2api:0.2.13`，容器、镜像及启动时间与更新前一致；本地 Core 工作树也保持干净。

验证：本地及 Linux 构建环境的 Portal 后端测试通过（未配置独立数据库的集成测试按原逻辑跳过）；前端账号检查相关 19 项测试通过。部署前通过现有管理员 API Key 的正常导出权限，运行新 Portal 适配器的真实上游验证：#746、#747 的同题均返回 `43`，不同题分别返回 `84`、`72`，#735 生成 4147 字节 HTML/SVG。该验证未使用浏览器会话；生产链路仍使用调用者 JWT，权限拒绝不会回退到服务器 Key。公开 Portal/Core 健康检查均通过。

## 验证

```powershell
cd portal/backend
go test ./...
cd ../frontend
pnpm exec vitest run src/components/account/__tests__/AccountCheckManagement.spec.ts src/views/admin/__tests__/AccountChecksView.spec.ts src/api/admin/__tests__/accountChecks.spec.ts
pnpm build
```

手动验收：管理员选择至少两个 OpenAI 账号，分别验证统一题目和不同题目；使用完整 HTML / SVG 动画题目检查各自的预览；一次选择 4 个账号验证排队与停止全部。普通用户、分组管理员和企业管理员不得访问检查 API。
- `GET /api/v1/admin/account-checks/history`：当前管理员各账号的最新记录摘要，用于恢复卡片，不下载历史 HTML。
- `GET /api/v1/admin/account-checks/accounts/{accountID}/history`：当前管理员该账号的最近 3 次完整记录。
