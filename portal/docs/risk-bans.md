# 用户触发风控自动封禁

实现仅位于 Portal 和入口 Nginx，不修改 Core 源码、镜像或数据库。封禁对象为错误记录的 `user_id`（客户用户），不是上游账号池的 `account_id`。

## 行为

- Portal 每 5 秒通过 Core 官方 `/api/v1/admin/ops/request-errors?view=all` 巡检；必须使用 `view=all`，默认视图会隐藏归为业务限制的风控事件（包括流式请求中 HTTP 200 携带的风控错误）。仅接受 `phase=request` 且类型精确为 `cyber_policy` 或 `cyber_policy_session_blocked` 的事件。普通 403、鉴权失败、限流、余额不足、错误文本中出现关键词均不会直接触发封禁。
- 通过 `/api/v1/admin/users/{id}/api-keys` 分页读取该用户全部 Key，在 Portal 保存 SHA-256 摘要与事件 ID，再通过 `PUT /api/v1/admin/users/{id}` 设置 `status=disabled`。Core 原生用户状态检查负责阻止全部 Key 的后续调用，包括绕过 Portal 的内部调用。
- Portal 中间件及公网 Nginx `auth_request` 在请求进入 Core 前拦截已记录的 Key，返回 HTTP 403：`error.code=ACCOUNT_RISK_BANNED`，`error.message=账号因触发风控已被禁止使用，请联系管理员。` 不缓存 Key 明文，不缓冲流式响应，不修改 WebSocket 数据。
- 首次启用回查最近 24 小时，按每批 1 小时推进；通常约两分钟完成历史补扫，实际耗时取决于记录量及 Core 延迟。随后继续持久化游标，每次回看 5 分钟以覆盖异步日志写入。处理失败不会前移游标，重启会重试；多个 Portal 实例通过 PostgreSQL advisory lock 避免重复扫描。
- 事件 ID 防止重复封禁。管理员在现有用户管理页面重新启用用户后，下一轮巡检清除入口拦截。已处理事件不会重复执行；新的风控事件仍会再次封禁。
- Core 不允许禁用管理员，管理员风控事件会记录告警供人工处理，不调用禁用接口。

日志采集必须启用，且日志未丢失。检测延迟包含约 5 秒巡检周期及 Core 写入延迟，不能阻止发现事件之前已经发出的并发请求，也不会终止已建立的流/连接。延迟写入超过 5 分钟的记录不保证补扫。首次启用前超过 24 小时的历史事件不自动追溯。

## 部署

1. 备份 Portal 数据库，构建并更新 Portal API。启动会幂等应用 `028_risk_bans.sql`，只创建 Portal 表。
2. 配置 `PORTAL_DATABASE_URL` 与 `CORE_ADMIN_API_KEY`，管理员 Key 必须允许读取错误记录、用户及用户 Key，并允许更新用户状态。`PORTAL_RISK_BAN_ENABLED` 默认 `true`，Compose 已透传。
3. 确认 Core 运维监控已启用，Portal 日志没有 `risk ban scan failed`。先验证通过 Portal Web 的 `/api/v1/internal/risk-check` 无凭据请求返回 204。
4. 将 `portal/deploy/nginx-sub2api-production.conf` 中 `/_portal_risk_check`、`@risk_banned` 及 Core 代理 location 内的 `auth_request` / `error_page` 合入实际站点配置。Nginx 需包含 `http_auth_request_module`。执行 `nginx -t` 成功后 reload。不要覆盖线上其他 location。若使用独享入口配置，也应保留相同检查或确保请求全部通过 Portal 中间件。
5. 使用专用测试用户及合成 Core 管理接口响应验证封禁，不通过发送违规内容制造真实风控。普通用户模型调用和流式/WebSocket 连接应保持正常；被封禁测试用户的全部 Key 均应返回上述 403。

未更新 Nginx 时，Core 用户禁用仍生效，但直连 Core 的客户端会看到 Core 原有 `USER_INACTIVE`，不会显示定制风控原因。数据库或风控检查服务不可用时，入口拒绝放行，返回服务错误而非误报封禁。内部 `/api/v1/internal/risk-check` 只做状态判断，不执行封禁，也不接受客户端指定用户 ID。

回滚时先移除 Nginx `auth_request`，再设置 `PORTAL_RISK_BAN_ENABLED=false` 并更新 Portal；已在 Core 禁用的用户不会自动解禁，需管理员审核后重新启用。保留 Portal 事件表便于审计。

## 验证

`cd portal/backend && go test ./...`

新增测试覆盖精确事件匹配、三种 Key 认证方式与优先级、全部 Key 持久化后禁用、Core 写入失败保留入口拦截并允许重试、重复事件幂等、管理员保护、人工解禁、用户删除、日志分页与失败时游标不前移。数据库交互使用 SQL mock；上线仍需执行迁移和 Nginx 配置验证。

## 2026-10-30 上线验证

东京生产已更新 Portal API 与 Nginx，Core 和其他 12 个容器身份及启动时间均保持不变。历史补扫完成：51 条风控事件、1 名用户、5 个 Key 摘要；验证时游标落后 1 秒，无扫描失败日志。该用户已被 Core 原生禁用，全部 5 个 Key 经公网返回 `403 ACCOUNT_RISK_BANNED`；另外两种 Key Header 和 Portal 入口同样返回定制提示。正常用户只读模型列表返回 200，匿名请求返回 401，Portal/Core/Imgtool 公网健康检查返回 200。未发起付费推理或人为制造风控。

备份：`/opt/backups/portal-risk-bans-20261030/`；发布目录：`/opt/releases/portal-risk-bans-20261030/`；新镜像：`portal-portal-api:risk-bans-20261030`；回滚镜像：`portal-portal-api:before-risk-bans-20261030`。部署及验证脚本位于本仓库 `artifacts/risk-bans-20261030/`（本地运维材料，可能不纳入 Git）。
