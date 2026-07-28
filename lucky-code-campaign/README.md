# 拼手气抢兑换码

一个独立、轻量的兑换码随机领取活动。单个 Go 进程同时提供用户页面、管理员页面和 API，数据保存在 SQLite 中。

## 功能

- 用户输入邮箱随机领取一枚兑换码
- 同一邮箱重复访问时返回第一次领取的兑换码
- SQLite 事务保证并发领取不重复
- 管理员批量粘贴或上传 TXT/CSV 导入兑换码
- 查看奖池总数、已领取数和剩余库存
- 搜索、分页查看领取记录并导出 CSV
- 管理员页面使用环境变量密码登录

## 本地运行

需要 Go 1.24 或更高版本。

```powershell
$env:ADMIN_PASSWORD='请替换为强密码'
go run .
```

打开：

- 活动页面：<http://localhost:8090/gift/>
- 管理页面：<http://localhost:8090/gift/admin>

数据默认保存在 `data/activity.db`。

## HTTP 接口

页面直接调用以下真实接口，没有前端模拟数据：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/gift/api/status` | 查询剩余库存 |
| `POST` | `/gift/api/claim` | 使用邮箱随机领取兑换码 |
| `POST` | `/gift/api/admin/login` | 管理员登录 |
| `POST` | `/gift/api/admin/logout` | 管理员退出 |
| `GET` | `/gift/api/admin/stats` | 查询奖池统计 |
| `POST` | `/gift/api/admin/codes/import` | 批量导入兑换码 |
| `GET` | `/gift/api/admin/claims` | 分页查询领取记录 |
| `GET` | `/gift/api/admin/claims/export` | 导出领取记录 CSV |

管理员接口使用 HttpOnly 会话 Cookie 保护。领取和导入数据均写入 SQLite，服务重启后不会丢失。

## 配置

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `ADMIN_PASSWORD` | 无 | 必填，管理员登录密码 |
| `ADDR` | `:8090` | HTTP 监听地址 |
| `DB_PATH` | `data/activity.db` | SQLite 数据库路径 |
| `COOKIE_SECURE` | `false` | 使用 HTTPS 部署时设为 `true` |

## 兑换码文件

TXT 文件每行一枚兑换码：

```text
CODE-8F3K-21MZ
CODE-6Q9P-47AX
```

CSV 文件读取第一列，可带 `code` 表头：

```csv
code,label
CODE-8F3K-21MZ,第一批
CODE-6Q9P-47AX,第一批
```

重复码会自动跳过，已经领取的记录不会因后续导入受到影响。

## 验证

```powershell
go test ./...
go build .
```

## 生产部署

正式入口固定为：

- 用户页面：<https://togoapi.com/gift>
- 管理页面：<https://togoapi.com/gift/admin>

应用独立监听 `127.0.0.1:18083`，由现有 `togoapi.com` Nginx 转发 `/gift` 路径。部署文件位于 `deploy/`：

- `lucky-code-campaign.service`：systemd 服务
- `activity.env.example`：生产环境变量样例
- `nginx-gift.locations.conf`：加入现有 HTTPS server 块的路径配置
- `nginx-gift-rate-limit.conf`：加入 Nginx `http` 上下文的限流区配置

Linux 构建：

```powershell
$env:GOOS='linux'
$env:GOARCH='amd64'
$env:CGO_ENABLED='0'
go build -trimpath -ldflags='-s -w' -o lucky-code-campaign-linux-amd64 .
```
