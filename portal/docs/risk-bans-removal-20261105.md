# 取消用户自动封禁：2026-11-05 部署记录

- 东京 Portal API 于 11:39:32（Asia/Shanghai）启动，11:40:12 公网验证通过；healthy，零重启。
- 取消错误日志扫描、自动禁用用户和历史 Key 拦截。403、`cyber_policy`、`cyber_policy_session_blocked` 正常返回原始错误；旧 `PORTAL_RISK_BAN_ENABLED` 开关不再生效。
- Nginx 实际站点只移除风控专用 location、`auth_request` 和封禁错误页，保留原始错误透传与其他代理配置。Portal 保留无状态 204 兼容接口供旧版 Nginx 使用。
- 仅重建 Portal API 并重载 Nginx，其他 12 个容器（含 Core、Portal Web、数据库、独立实例）的容器 ID、镜像、启动时间、重启次数均未变化。
- 未新增数据库迁移、未清理历史风控记录、未修改任何用户状态。历史记录关联的用户 5、362 在部署前后均为 active。

## 验证

- 本地 Windows 与服务器隔离 Linux 构建容器的 `go test ./...` 均通过。未配置测试数据库或真实上游，对应集成测试按现有规则跳过。
- 新回归覆盖三种错误原样返回及同一用户下一次正常请求成功，兼容接口覆盖无凭据和 Bearer / X-Api-Key / X-Goog-Api-Key，GET/POST 均返回空响应 204。
- 公网 Portal/Core/Imgtool 健康检查均 200；正常 Key 模型列表 200；匿名模型列表 401。历史两个用户各选一个 Key、三种 Header，公网状态码及响应体均与 Core 原生响应一致，无 `ACCOUNT_RISK_BANNED` 拦截。
- Nginx 配置校验和重载通过（保留原有非致命 protocol options 警告）；启动日志无 ERROR/FATAL/panic 或封禁扫描记录。未发送付费推理请求。
- 发布源码哈希、删除的封禁源码文件、备份哈希、运行环境开关移除均已验证。

## 发布与备份

- 基线提交：`bd15bb1`；部署内容来自本次尚未提交的工作区变更。
- 镜像：`portal-portal-api:no-risk-bans-20261105`。
- 镜像 ID：`sha256:fb972f7287d0b8c63af1aa5b047f3e436bfc6490bf7c17177aa7ee8b2e1cd694`。
- 发布目录：`/opt/releases/disable-risk-bans-20261105/`，包含测试/构建日志、源码、manifest 和 verification.json。
- 备份：`/opt/backups/disable-risk-bans-20261105/`，包含 3,326,325 字节 PostgreSQL custom dump、可读恢复列表、源码、Nginx、Compose、受保护的环境快照和 SHA-256 校验。未演练完整数据库恢复。
- 旧镜像标签：`portal-portal-api:before-no-risk-bans-20261105`。
- 发布包 SHA-256：`800ec819c9153a6dcb983b6f40363257b692fee1f332a3811a65c194474be015`。
- 本地脚本：`artifacts/disable-risk-bans-20261105/`。备份和凭据留在服务器。

## 回滚

旧镜像默认会自动封禁，不能直接恢复旧镜像和配置。应继续保留已移除风控检查的 Nginx 配置，在 Compose 的 portal-api.environment 中明确设置 `PORTAL_RISK_BAN_ENABLED: "false"`，再把旧镜像重新标记为 `portal-portal-api:latest`，使用现有两个 Compose 文件和 `.env`、`up -d --no-deps --no-build --wait portal-api` 仅重建 API。

代码回滚从受保护的 source.tar.gz 只恢复 manifest 中列出的变更和删除文件，保留后续改动；新测试文件仅在内容仍匹配发布版本时删除。无需恢复数据库或重启 Core。回滚后检查正常 Key、匿名请求及健康检查；旧镜像关闭开关后不提供兼容接口，因此必须保持 Nginx 风控检查已移除。
