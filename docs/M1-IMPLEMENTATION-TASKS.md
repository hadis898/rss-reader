# RSS Reader M1 实施任务拆解

本清单对应 `docs/OPTIMIZATION-DESIGN.md` 的 M1 里程碑，目标是 1 周内完成可上线的第一阶段优化。

## 一、目标

- 建成可用的管理员后台（配置管理 + 源测试）
- 落地首页 UI 第一轮优化（结构更清晰、交互更可控）
- 增加基础运维接口（健康检查）
- 保持部署方式不变（Docker 兼容）

## 二、任务拆解（按优先级）

## P0：必须完成

1. 管理员后台增强
- [x] 新增后台页面：`/admin`
- [x] 配置读写接口：`GET/PUT /api/admin/config`
- [x] 单源测试接口：`POST /api/admin/test-source`
- [x] Token 鉴权（`Authorization: Bearer` 与 `X-Admin-Token`）
- [x] 支持 `ADMIN_TOKEN` 环境变量覆盖
- [ ] 增加防爆破限制（后续可做简单 IP 限流）

2. 配置模型与校验
- [x] `Config` 新增 `adminToken`
- [x] 增加配置校验函数 `Validate()`
- [x] 增加 `SaveConf()` 安全写回
- [ ] 增加 URL 格式与协议校验（http/https）

3. 首页 UI 第一轮优化
- [ ] 增加顶部工具栏（搜索、主题、刷新状态）
- [ ] 增加 Feed 名称快速过滤
- [ ] 增加“仅看未读”（先前端状态实现）
- [ ] 卡片视觉层级统一（间距、字号、边框）

4. 基础运维
- [ ] 增加 `GET /api/health`
- [ ] 返回最近一次抓取状态与错误计数

## P1：建议完成

1. 接口文档与 README
- [x] README 补充管理员后台说明
- [ ] 增加 API 示例（curl）
- [ ] 增加安全建议（强 Token、反代鉴权）

2. 管理后台体验优化
- [x] 表单化增删 RSS 源
- [x] JSON 预览/回填
- [x] 操作日志面板
- [ ] 来源排序（上移/下移）

## 三、接口定义（M1）

## 1) 获取配置

- Method: `GET`
- Path: `/api/admin/config`
- Header: `Authorization: Bearer <token>`
- Response:

```json
{
  "values": ["https://example.com/rss"],
  "refresh": 6,
  "autoUpdatePush": 7,
  "nightStartTime": "06:30:00",
  "nightEndTime": "19:30:00",
  "hasAdminToken": true
}
```

## 2) 保存配置

- Method: `PUT`
- Path: `/api/admin/config`
- Header: 同上
- Body:

```json
{
  "values": ["https://example.com/rss"],
  "refresh": 6,
  "autoUpdatePush": 7,
  "nightStartTime": "06:30:00",
  "nightEndTime": "19:30:00"
}
```

- 成功返回：`204 No Content`

## 3) 测试源地址

- Method: `POST`
- Path: `/api/admin/test-source`
- Header: 同上
- Body:

```json
{
  "url": "https://example.com/rss"
}
```

- 成功返回：

```json
{
  "title": "Example Feed",
  "items": 12
}
```

## 四、验收标准（M1）

- 访问 `/admin` 可完成配置加载、编辑、保存
- 错误 token 访问管理员接口返回 401
- 配置保存后页面数据自动刷新并生效
- 单源测试失败时可见明确错误信息
- README 与设计文档同步更新

## 五、上线检查清单

- [ ] `adminToken` 已替换为强随机字符串
- [ ] 生产环境优先使用 `ADMIN_TOKEN` 环境变量
- [ ] 反代层（Nginx/Caddy）限制 `/admin` 访问来源
- [ ] 对外服务启用 HTTPS
- [ ] 备份 `config.json`
