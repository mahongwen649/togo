# TogoAPI Lottery Campaign Frontend

独立的多期随机额度活动前端，包含用户端和管理端。活动、报名、开奖和发放记录均通过 Portal 服务读取与写入，中奖额度由服务负责计算并通过 Core 发放。

## 本地运行

```powershell
npm install
npm run dev -- --host 127.0.0.1
```

- 用户端：`http://127.0.0.1:5173/user-web/`
- 管理端：`http://127.0.0.1:5173/admin-web/`

用户端只能使用 Core 中已注册且有效的账户邮箱报名。没有活动、服务不可用或结果尚未生成时，页面会显示对应的真实状态。

## 验证

```powershell
npm run build
```

## 生产镜像

```powershell
docker build -t togoapi-lottery .
docker run --rm -p 8088:8080 -e PORTAL_API_UPSTREAM=http://host.docker.internal:3000 togoapi-lottery
```

生产环境应让 `PORTAL_API_UPSTREAM` 指向 Portal API，并由外层 HTTPS 入口代理该容器。默认生产链接为：

- 用户端：`https://togoapi.com/lottery/user-web/`
- 管理端：`https://togoapi.com/lottery/admin-web/`

该活动使用独立链接访问，不需要加入 Portal 导航。生产 Compose 和 Nginx 路由示例位于 `deploy/`。
