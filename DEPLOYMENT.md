# TogoAPI 服务器与部署速查

新建独立站点 [ai.togoapi.com](https://ai.togoapi.com)：本地项目为 `E:\code\ai-sub2api`，服务器目录为 `/opt/ai-sub2api`，容器名称为 `ai-sub2api` / `ai-postgres` / `ai-redis`。已于 2026-10-20 上线，公网健康检查和管理员登录通过；首次登录需本人确认官方运营承诺，尚未配置上游账号。详情见 [新项目部署说明](../ai-sub2api/DEPLOYMENT.md)。

更新时间：2026-10-29（Asia/Shanghai）

本文记录服务器登录、线上入口、部署目录和日常运维命令。密码、私钥、Admin API Key、数据库密码和应用密钥不写入仓库；它们只保存在服务器的 `.env` 或本机密钥目录中。

## 1. 服务器登录

### 当前生产服务器：东京

| 项目 | 值 |
| --- | --- |
| SSH 别名 | `togoapi-tokyo` |
| 地址 | `43.165.190.2` |
| 用户 / 端口 | `ubuntu` / `22` |
| 本机私钥 | `C:\\Users\\Administrator\\.ssh\\id_ed25519_togoapi_sg` |
| 云主机 | Tencent Cloud Lighthouse，Tokyo zone 2 |
| 规格 | Ubuntu 24.04.4 LTS，4 核 / 8 GB / 120 GB |

PowerShell：

```powershell
ssh togoapi-tokyo
```

服务器上的主要入口：

```bash
cd /opt/sub2api       # Core
cd /opt/portal        # Portal
cd /opt/newsub        # 独立 NewSub
cd /opt/imgtool       # Imgtool
cd /opt/lucky-code-campaign
cd /opt/lottery-campaign
```

### 回滚与历史服务器

| 服务器 | SSH 命令 | 用途 | 当前结论 |
| --- | --- | --- | --- |
| 新加坡 `43.160.240.196` | `ssh togoapi-sg` | 迁移回滚点 | 应用已停止，保留 PostgreSQL；2026-10-20 连接被服务器主动关闭 |
| 美国 `170.106.190.46` | `ssh togoapi-us` | 历史主机 | 非生产、非回滚；当前 SSH 主机指纹与本机 `known_hosts` 不一致，连接前必须人工核验 |

不要在东京和新加坡同时启动会写入生产数据的应用服务。

## 2. 公网入口

| 地址 | 用途 |
| --- | --- |
| https://togoapi.com | Portal 主站 |
| https://api.togoapi.com | Core API、管理员入口；模型 API 基址为 `https://api.togoapi.com/v1` |
| https://img.togoapi.com | Imgtool 图片生成站点和 API |
| https://newsub.43-165-190-2.sslip.io | 独立 NewSub 实例 |
| https://togoapi.com/gift/ | Lucky-code 活动页 |
| https://togoapi.com/gift/admin | Lucky-code 管理页 |
| https://togoapi.com/lottery/ | Lottery 活动页 |

Cloudflare 的 `@`、`api`、`img` 三个 A 记录均代理到东京 `43.165.190.2`。数据库和 Redis 只允许 Docker 内网访问。

## 3. 线上服务映射

| 服务 | 部署位置 | 运行方式 | 本机监听 | Nginx / systemd |
| --- | --- | --- | --- | --- |
| Core | `/opt/sub2api` | Docker Compose | `127.0.0.1:8080` | `/etc/nginx/sites-available/sub2api` |
| Portal | `/opt/portal` | 两个 Compose 文件叠加 | `127.0.0.1:3000` | 与 Core 共用站点转发 |
| NewSub | `/opt/newsub` | Docker Compose | `127.0.0.1:8081` | `/etc/nginx/sites-available/newsub` |
| Imgtool | `/opt/imgtool` | systemd + 前端静态文件 | `127.0.0.1:18082` | `/etc/nginx/sites-available/imgtool`、`imgtool.service` |
| Lucky-code | `/opt/lucky-code-campaign` | systemd | `127.0.0.1:18083` | `lucky-code-campaign.service` |
| Lottery | `/opt/lottery-campaign/deploy` | Docker Compose | `127.0.0.1:18084` | Nginx `/lottery/` |

2026-10-20 线上检查：Core、Portal、NewSub、Lottery、Imgtool、Lucky-code 和 Nginx 均为 active/healthy；公网检查返回 200。根文件系统 118 GB 中已用约 63 GB（56%）。

## 4. 常用检查命令

```bash
ssh togoapi-tokyo

# Core
cd /opt/sub2api
docker compose --env-file .env -f docker-compose.local.yml ps
docker compose --env-file .env -f docker-compose.local.yml logs --tail 200 sub2api
curl -fsS http://127.0.0.1:8080/health

# Portal
cd /opt/portal
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env ps
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env logs --tail 200 portal-api portal-web
curl -fsS http://127.0.0.1:3000/health

# Imgtool / Lucky-code / Nginx
sudo systemctl status imgtool.service lucky-code-campaign.service --no-pager
curl -fsS http://127.0.0.1:18082/api/health
curl -fsS http://127.0.0.1:18083/gift/api/status
sudo nginx -t
sudo systemctl reload nginx

# 端口、磁盘和容器
sudo ss -lntp | grep -E ':(22|80|443|3000|8080|8081|18082|18083|18084) '
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
df -h /
```

公网快速检查（本机 PowerShell）：

```powershell
curl.exe -I https://togoapi.com/
curl.exe -I https://api.togoapi.com/health
curl.exe -I https://api.togoapi.com/admin
curl.exe -I https://img.togoapi.com/api/health
curl.exe -I https://newsub.43-165-190-2.sslip.io/health
curl.exe -I https://togoapi.com/gift/
curl.exe -L -I https://togoapi.com/lottery/
```

## 5. 部署原则与流程

### Core

Core 使用 `/opt/sub2api/docker-compose.local.yml` 和 `/opt/sub2api/.env`，当前镜像为 `weishaw/sub2api:0.2.19`（2026-10-29 已升级，500K 计费覆盖规则保留）。升级前必须先备份数据库和部署文件，并保留旧镜像作为回滚版本：

```bash
cd /opt/sub2api
backup_dir="/opt/backups/sub2api-core-upgrade-$(date +%Y%m%d-%H%M%S)"
sudo mkdir -p "$backup_dir" && sudo chown ubuntu:ubuntu "$backup_dir"
set -a; . ./.env; set +a
: "${POSTGRES_USER:=sub2api}"; : "${POSTGRES_DB:=sub2api}"
docker compose --env-file .env -f docker-compose.local.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" | gzip > "$backup_dir/postgres.sql.gz"
tar --exclude='./postgres_data' --exclude='./redis_data' --exclude='./.env' \
  -czf "$backup_dir/sub2api-deploy-files.tgz" .
chmod 600 "$backup_dir"/*
docker compose --env-file .env -f docker-compose.local.yml pull sub2api
docker compose --env-file .env -f docker-compose.local.yml up -d sub2api
docker compose --env-file .env -f docker-compose.local.yml ps
curl -fsS http://127.0.0.1:8080/health
```

### Portal

```bash
cd /opt/portal
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env up -d --build
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env ps
```

### Imgtool、Lucky-code、Lottery

Imgtool 和 Lucky-code 通过 systemd 管理，修改二进制或配置后使用 `sudo systemctl restart <unit>`，然后检查 `journalctl -u <unit> -n 100 --no-pager`。Lottery 使用 `/opt/lottery-campaign/deploy/docker-compose.production.yml`，容器绑定 `127.0.0.1:18084`。

禁止执行 `docker compose down -v`，禁止删除 Core、NewSub 或 Portal 的数据库/Redis 数据目录和 Docker volume。

## 6. 证书和待办

2026-10-23 已修复主站证书过期导致的 Cloudflare 526，并将主站/API 和 Imgtool 改为 HTTP-01 webroot 自动续期：

| 域名 | 证书路径 | 到期时间（服务器检查值） |
| --- | --- | --- |
| `api.togoapi.com`、`togoapi.com` | `/etc/letsencrypt/live/togoapi.com/` | 2027-01-21 09:03:40 UTC+8 |
| `img.togoapi.com` | `/etc/letsencrypt/live/img.togoapi.com/` | 2027-01-21 09:04:27 UTC+8 |
| `newsub.43-165-190-2.sslip.io` | `/etc/letsencrypt/live/newsub.43-165-190-2.sslip.io/` | 2026-12-19 |

主站/API 和 Imgtool 的 HTTP `/.well-known/acme-challenge/` 直接读取 `/var/www/togoapi-acme`，其他 HTTP 请求继续跳转 HTTPS。`certbot.timer` 已启用且 active，`/etc/letsencrypt/renewal-hooks/deploy/reload-nginx` 会检查并平滑重载 Nginx。不要将 Nginx 证书路径改回旧的静态副本 `/etc/nginx/ssl/togoapi/`。

手动演练：`sudo certbot renew --cert-name togoapi.com --dry-run --run-deploy-hooks`；Imgtool 使用 `--cert-name img.togoapi.com`。故障前配置、旧主站证书和容器身份基线保存在 `/opt/backups/origin-tls-20261023-020132/`。旧主站证书已过期，不可直接恢复用于线上。

2026-10-23 两组证书的续期演练和 Nginx 重载钩子均通过；公网主站、充值管理页、Core 后台和相关健康检查均返回 200，13 个容器的身份、镜像、启动时间和重启次数保持一致。Certbot 中另有既存的 `ppt.togoapi.com` 续期失败记录，本次未修改该独立证书。

## 7. 安全与敏感信息

- SSH 私钥只从本机 `C:\\Users\\Administrator\\.ssh\\` 使用，不能复制到仓库或服务器部署目录。
- Core、Portal、NewSub、Imgtool、Lucky-code 的 `.env` 保持服务器端 `600` 权限，不要在日志、工单或聊天中打印。
- Core 管理员密码、Admin API Key、SMTP 密码、COS 密钥和 Imgtool SSO secret 均不记录在本文。
- 东京公网 Web 入口经过 Cloudflare；完成回滚窗口后，应把 80/443 收敛到 Cloudflare 官方源地址范围，并把 SSH 22 限制为可信管理地址。
- `/opt/backups/` 和 `/opt/releases/` 中可能包含数据库备份或发布材料，操作时不要下载到本地仓库。
