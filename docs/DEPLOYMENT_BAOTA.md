# 25code 生产部署文档（宝塔 + Docker）

本文适用于已经安装 Docker、Docker Compose 和宝塔面板的 Ubuntu、Debian、CentOS 等服务器。

## 1. 部署结构

~~~
用户浏览器
    |
宝塔 Nginx（HTTPS，80/443）
    |
127.0.0.1:2100（Docker web）
    |
Go API + PostgreSQL + Redis + RustFS
~~~

2100 只绑定本机，不需要对公网开放。公网只开放宝塔的 80 和 443。

## 2. 前置检查

服务器建议至少 2 核 CPU、4 GB 内存、40 GB 可用磁盘：

~~~
docker --version
docker compose version
~~~

域名需要先解析到服务器公网 IP，例如：

~~~
ai.example.com  ->  服务器公网 IP
~~~

## 3. 拉取项目

~~~
sudo mkdir -p /opt
cd /opt
sudo git clone https://github.com/kimihalle/image2api-saas.git 25code
sudo chown -R "$USER":"$USER" /opt/25code
cd /opt/25code
~~~

后续更新：

~~~
cd /opt/25code
git pull --ff-only origin main
~~~

## 4. 配置生产环境

不要直接修改 .env.example，复制一份服务器专用配置：

~~~
cd /opt/25code
cp .env.example .env
chmod 600 .env
nano .env
~~~

至少修改以下配置：

~~~
WEB_PORT=2100
APP_TITLE=25code
PUBLIC_ORIGIN=https://ai.example.com
COOKIE_SECURE=true

POSTGRES_DB=northstar
POSTGRES_USER=northstar
POSTGRES_PASSWORD=替换成至少32位随机数据库密码

RUSTFS_ACCESS_KEY=替换成随机访问密钥
RUSTFS_SECRET_KEY=替换成至少32位随机存储密钥

BOOTSTRAP_ADMIN_EMAIL=admin@example.com
BOOTSTRAP_ADMIN_PASSWORD=满足复杂度要求的首次管理员密码
GENERATION_WORKERS=8
~~~

注意：

- PUBLIC_ORIGIN 必须和用户访问的 HTTPS 域名完全一致，不要写结尾斜杠。
- 首次管理员密码必须包含大小写字母、数字和符号，长度 8–24 位。
- 首次启动成功创建管理员后，从 .env 删除 BOOTSTRAP_ADMIN_EMAIL 和 BOOTSTRAP_ADMIN_PASSWORD，然后重新启动服务。
- PostgreSQL 数据卷初始化后，修改 .env 的 POSTGRES_PASSWORD 不会自动修改数据库内部密码，不要随意改。
- 随机密钥可用以下命令生成：

~~~
openssl rand -base64 36
openssl rand -hex 32
~~~

## 5. 启动 Docker

~~~
cd /opt/25code
docker compose pull
docker compose up -d --build
docker compose ps
~~~

首次启动会编译 Go 后端和 Vue 前端，可能需要几分钟。

本机健康检查：

~~~
curl -fsS http://127.0.0.1:2100/health/live
~~~

管理员入口：

~~~
https://ai.example.com/admin/overview
~~~

普通用户前台入口：

~~~
https://ai.example.com/
~~~

## 6. 宝塔配置域名和 HTTPS

1. 宝塔“网站”中添加域名 ai.example.com。
2. 进入“SSL”，申请 Let's Encrypt 证书并开启强制 HTTPS。
3. 进入站点“配置文件”，将站点根路径代理到 Docker web。
4. 保存并重载 Nginx。

代理配置：

~~~nginx
location / {
    proxy_pass http://127.0.0.1:2100;
    proxy_http_version 1.1;

    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    proxy_connect_timeout 60s;
    proxy_send_timeout 600s;
    proxy_read_timeout 600s;
    proxy_buffering off;
    client_max_body_size 50m;
}
~~~

proxy_buffering off 和长超时用于实时状态、长任务和大文件请求，不能省略。

## 7. 首次运营配置

登录后台后建议按以下顺序配置：

1. 系统设置：站点名称、Logo、联系信息、SMTP、支付参数。
2. Provider 账号：导入上游凭证，等待账号资料和额度同步。
3. 模型目录：启用模型并配置用户价格、代理价格、能力限制。
4. 视频模型：配置三宝 Key、时长、分辨率和价格。
5. 违禁词管理：确认图片拦截和政治类自动封禁策略。
6. 充值套餐与兑换码：配置金额、额度、支付方式。
7. 首页内容、公告和灵感模板：确认内容后再发布。
8. 用普通用户账号完成注册、充值、图片生成、失败退款和兑换码测试。

## 8. 更新版本

~~~
cd /opt/25code
git pull --ff-only origin main
docker compose up -d --build
docker compose ps
~~~

查看日志：

~~~
docker compose logs --tail=100 backend
docker compose logs --tail=100 web
~~~

不要在生产环境执行：

~~~
docker compose down -v
~~~

-v 会删除 PostgreSQL、Redis、RustFS 等数据卷，可能造成用户、账单和作品永久丢失。

## 9. 数据库和作品备份

### PostgreSQL

~~~
cd /opt/25code
mkdir -p backups
docker compose exec -T postgres pg_dump -U northstar -d northstar \
  > "backups/northstar-$(date +%F-%H%M%S).sql"
~~~

备份文件要同步到另一台服务器或对象存储，不要只保存在当前服务器。

### RustFS 作品和上传文件

先查看数据卷名称：

~~~
docker volume ls | grep rustfsdata
~~~

将下面的卷名替换成实际名称：

~~~
docker run --rm \
  -v image2api-saas_rustfsdata:/data:ro \
  -v /opt/25code/backups:/backup \
  alpine:3.20 tar czf /backup/rustfs-$(date +%F-%H%M%S).tar.gz -C /data .
~~~

## 10. 恢复数据库

恢复前先备份当前数据库，并停止后端：

~~~
cd /opt/25code
docker compose stop backend
cat backups/northstar-YYYY-MM-DD-HHMMSS.sql | \
  docker compose exec -T postgres psql -U northstar -d northstar
docker compose start backend
~~~

如果目标数据库不是空库，先确认表覆盖策略。不要在没有备份的情况下执行 DROP SCHEMA public CASCADE。

## 11. 常见故障

### 502 Bad Gateway

~~~
docker compose ps
docker compose logs --tail=200 backend
~~~

确认 backend 为 healthy，并确认宝塔代理地址是 127.0.0.1:2100。

### 登录后反复跳回首页

检查：

~~~
PUBLIC_ORIGIN=https://ai.example.com
COOKIE_SECURE=true
~~~

如果是 HTTP 本机测试，使用 COOKIE_SECURE=false。生产环境必须使用 HTTPS。

### 上传或生成大文件失败

确认宝塔 Nginx 有 client_max_body_size 50m、proxy_read_timeout 600s 和 proxy_buffering off，并查看：

~~~
docker compose logs --tail=100 rustfs
docker compose logs --tail=100 backend
~~~

### PostgreSQL 或 Redis 不健康

~~~
docker compose ps
docker compose logs --tail=200 postgres
docker compose logs --tail=200 redis
~~~

不要删除数据卷来处理健康检查问题，先检查磁盘空间、密码和容器网络。

### 生成任务长时间没有结果

检查 Provider 账号状态、模型启用状态、上游代理和额度：

~~~
docker compose logs --tail=300 backend
~~~

GENERATION_WORKERS 控制并行 worker 数量。账号池不足时不要盲目调大，否则会增加上游限流和失败率。

## 12. 上线前检查清单

- [ ] 域名 HTTPS 正常，/health/live 返回成功。
- [ ] .env 没有示例密码，权限为 600。
- [ ] Bootstrap 管理员密码已从运行环境移除。
- [ ] PostgreSQL、Redis、RustFS 数据卷均已创建并有备份。
- [ ] 注册、登录、找回密码正常。
- [ ] 图片生成成功、失败退款、生成记录更新正常。
- [ ] 视频生成成功、失败状态和下载正常。
- [ ] 支付回调、订单状态和额度到账已实测。
- [ ] 兑换码生成、兑换、删除和批量删除已实测。
- [ ] SMTP、支付、Provider 和模型价格已配置。
- [ ] 宝塔 Nginx 已配置 50 MB 上传限制、600 秒超时和关闭代理缓冲。
- [ ] 已设置每日数据库和作品备份，并验证过一次恢复流程。

