# 风控错误正常返回，不再自动封禁用户

## 当前行为（2026-11-05）

- 普通 403、`cyber_policy` 和 `cyber_policy_session_blocked` 都只作为当前请求的错误返回，保留 Core 的状态码和响应内容。Portal 不再扫描错误日志、读取用户全部 Key 或调用管理 API 自动设置用户 `status=disabled`。
- 已移除自动封禁后台任务和 Portal Key 拦截中间件。旧环境中的 `PORTAL_RISK_BAN_ENABLED` 不再生效，Compose 也不再透传此开关。
- 生产 Nginx 配置模板移除 `/_portal_risk_check`、`@risk_banned` 以及对应的 `auth_request` / `error_page`。继续使用 `proxy_intercept_errors off`，上游错误正常返回，流式/WebSocket 转发设置保持原样。
- 为兼容尚未更新的 Nginx，Portal 始终保留 `/api/v1/internal/risk-check` 并返回空响应 204；不查询数据库或 Core。这个兼容接口不授权模型调用，实际请求仍由 Core 正常鉴权。
- 保留历史迁移 `028_risk_bans.sql` 和 Portal 风控记录用于审计，历史 Key 摘要不再用于入口拦截。此前已在 Core 被禁用的用户不会自动恢复；如需恢复，可在现有用户管理页面单独启用。

## 部署

1. 构建并更新 Portal API。即使旧环境保留 `PORTAL_RISK_BAN_ENABLED=true`，新版也不会启动自动封禁。
2. 从实际 Nginx 站点配置移除上述风控专用 location 和引用，保留其他路由与配置；执行 `nginx -t` 成功后 reload。兼容接口允许先更新 Portal、再更新 Nginx。
3. 从运行环境删除无效的 `PORTAL_RISK_BAN_ENABLED` 设置。确认公共健康检查、匿名请求鉴权和正常 Key 请求工作正常。
4. 验证 403/风控错误只影响当前请求，后续正常请求可以继续。使用合成 Core 响应，不通过发送违规内容制造真实风控。

东京生产已于 2026-11-05 11:39（Asia/Shanghai）完成更新，11:40 公网验证通过。仅重建 Portal API 并重载 Nginx，其余 12 个容器的身份、镜像和启动时间保持不变。历史记录涉及的用户 5、362 当前均为 `active`，部署未修改用户状态。详见 `risk-bans-removal-20261105.md`。恢复旧 Portal 镜像会重新引入自动封禁；如确需回滚，应先确认旧镜像配置 `PORTAL_RISK_BAN_ENABLED=false`，并已移除 Nginx 风控检查。

## 验证

`cd portal/backend && go test ./...`

回归测试覆盖旧 Nginx 检查在无凭据及三种 Key Header 下返回 204、GET/POST 方法兼容、普通 403 和两种风控错误原样返回，以及同一用户后续正常请求成功。未配置测试数据库或真实上游时，对应集成测试按现有规则跳过。

## 历史：2026-10-30 上线验证

当时东京生产启用了 Portal 自动封禁和 Nginx Key 拦截，Core 和其他 12 个容器身份及启动时间保持不变。历史补扫完成 51 条风控事件、1 名用户、5 个 Key 摘要；该用户被 Core 原生禁用，全部 5 个 Key 经公网返回 `403 ACCOUNT_RISK_BANNED`。此次代码变更取消该自动处理行为。

历史备份：`/opt/backups/portal-risk-bans-20261030/`；发布目录：`/opt/releases/portal-risk-bans-20261030/`；当时镜像：`portal-portal-api:risk-bans-20261030`；更早镜像：`portal-portal-api:before-risk-bans-20261030`。旧部署及验证脚本位于 `artifacts/risk-bans-20261030/`（本地运维材料，可能不纳入 Git）。
