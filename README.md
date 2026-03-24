# 简述

实时展示rss订阅最新消息。

## 特性

- 打包后镜像大小仅有约20MB，通过docker实现一键部署

- 支持自定义配置页面数据自动刷新

- 响应式布局，能够兼容不同的屏幕大小

- 良好的SEO，首次加载使用模版引擎快速展示页面内容

- 支持添加多个RSS订阅链接

- 简洁的页面布局，可以查看每个订阅链接最后更新时间

- 支持夜间模式

- config.json配置文件支持热更新

## 界面预览（当前版本）

- 首页：四列大卡、单行省略、白天/夜间模式、搜索过滤
- 管理后台：在线编辑 `config.json`、测试 RSS 源、通知参数管理

![](https://cdn.skyimg.net/up/2026/3/24/39957b9f.webp)


# 配置文件

配置文件位于config.json，sources是RSS订阅链接，示例如下

```json
{
    "values": [
        "https://blog.upx8.com/feed",
        "https://www.ahhhhfs.com/feed",
        "https://rss.nodeseek.com",
        "https://linux.do/latest.rss",
        "https://51.ruyo.net/feed",
        "https://fuliba2024.net/feed",
        "https://naiyous.com/feed",
        "https://blog.cmliussss.com/atom.xml",
        "https://appscross.com/feed",
        "https://www.cheshirex.com/feed",
        "https://www.notetoday.net/feed",
        "https://www.freedidi.com/feed"
    ],
    "refresh": 2,
    "autoUpdatePush": 7,
    "nightStartTime": "06:30:00",
    "nightEndTime": "19:30:00",
    
    "keywords": ["Claw","活动","白嫖","甲骨文","netcup","狐蒂","GCP"],
    "notify" : {
        "feishu": {
            "api": ""
        },
        "telegram": {
            "api": "https://api.telegram.org/bot${token}/sendMessage",
            "chat_id":"输入你的ID",
            "token": "输入你的token"
        }
    },
    "archives": "archives.txt"
}
```

名称 | 说明
-|-
values | rss订阅链接（必填）
keywords | 首页快捷关键词（可选），用于搜索联想和一键过滤
refresh | rss订阅更新时间间隔，单位分钟（必填）
autoUpdatePush | 自动刷新间隔，默认为0，不开启。效果为前端每autoUpdatePush分钟自动更新页面信息，单位分钟（非必填）
nightStartTime | 日间开始时间 ，如 06:30:00
nightEndTime | 日间结束时间，如 19:30:00
adminToken | 管理员后台令牌，不配置则后台接口不可用（建议改成强随机字符串）
notify.telegram | telegram 通知配置（api/chat_id/token）
notify.feishu | 飞书通知配置（api）
archives | 归档文件路径（如 archives.txt）

## 管理员后台

为了便于在线管理配置，新增管理员后台：
- 后台密码在 `docker-compose.yml`  里的 `ADMIN_TOKEN=` 修改 （默认密码`admin123456`）
- 后台地址：`/admin`
- 获取配置：`GET /api/admin/config`
- 保存配置：`PUT /api/admin/config`
- 测试单个源：`POST /api/admin/test-source`
- 鉴权方式：`Authorization: Bearer <token>` 或 `X-Admin-Token: <token>`

说明：

- `adminToken` 可以配置在 `config.json`，也可以通过环境变量 `ADMIN_TOKEN` 覆盖
- 若环境变量和配置文件都为空，管理员接口会返回 `401 unauthorized`
- 保存配置会写回 `config.json` 并立即重新加载配置（自动触发数据更新）
- 后台页面支持表单化增删源、JSON 预览/回填、单源连通性测试

# 使用方式

## Docker部署

环境要求：Git、Docker、Docker-Compose

克隆项目

```bash
git clone https://github.com/hadis898/rss-reader
```

进入rss-reader文件夹，运行项目

```bash
docker-compose up -d
```

如果要重新部署，命令是

```bash
docker-compose down
docker-compose up -d --build
```

国内服务器将Dockerfile中取消下面注释使用 go mod 镜像
```dockerfile
#RUN go env -w GO111MODULE=on && \
#    go env -w GOPROXY=https://goproxy.cn,direct
```

部署成功后，通过ip+端口号访问

## 前端调试模式（免重建镜像）

如果你频繁调 `globals/static/index.html` 或 `globals/static/admin.html`，可以用开发编排文件：

```bash
docker-compose -f docker-compose.dev.yml up -d --build
```

说明：

- 该模式启用 `DEV_STATIC_DIR=/app/static-dev`
- 容器会直接读取本机挂载的 `globals/static` 目录
- 前端文件修改后，浏览器刷新即可生效（无需重新 build 镜像）

停止开发模式：

```bash
docker-compose -f docker-compose.dev.yml down
```

# nginx反代

这里需要注意/ws，若不设置proxy_read_timeout参数，则默认1分钟断开。静态文件增加gzip可以大幅压缩网络传输数据

```conf
server {
    listen 443 ssl;
    server_name rss.xxxx.xyz;
    ssl_certificate  fullchain.cer;
    ssl_certificate_key lass.cc.key;
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;
    location / {
        proxy_pass  http://localhost:8080;
    }
    location /ws {
        proxy_pass http://localhost:8080/ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 300s;
    }
}

server {
    listen 80;
    server_name rss.xxxx.xyz;
    rewrite ^(.*)$ https://$host$1 permanent;
}
```
