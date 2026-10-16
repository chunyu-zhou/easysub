<div align="center">

# 省心订阅 EasySub

**自托管的订阅 / 续费 / 保号管理系统：集中记录服务订阅、续费日期和付款方式，通过多渠道提醒避免订阅过期或号码掉号。**

[![License](https://img.shields.io/github/license/chunyu-zhou/easysub?color=blue)](./LICENSE)
[![GHCR](https://img.shields.io/badge/GHCR-ghcr.io%2Fchunyu--zhou%2Feasysub-2496ED?logo=github)](https://github.com/chunyu-zhou/easysub/pkgs/container/easysub)
[![Latest commit](https://img.shields.io/github/last-commit/chunyu-zhou/easysub/main?logo=github)](https://github.com/chunyu-zhou/easysub/commits/main)

```bash
docker run -d --name easysub \
  -p 8842:8000 \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD=replace-with-a-long-unique-password \
  -e ADMIN_EMAIL=admin@example.com \
  -e JWT_SECRET="$(openssl rand -hex 32)" \
  -v easysub_data:/app/data \
  --restart unless-stopped \
  ghcr.io/chunyu-zhou/easysub:latest
```

</div>

## ✨ 功能特性

| 分类 | 功能 |
| --- | --- |
| 👥 **多用户与权限** | JWT 登录、管理员 / 普通用户分层、数据按用户隔离、可开放注册，并可启用管理员审核。 |
| 💳 **订阅管理** | 支持周期订阅和一次性买断；自动计算下次续费日期；支持提前续费或按原到期日续费。 |
| 📱 **保号场景** | 续费时可从当前日期重新计算周期，适用于 SIM 卡保号、流量包和运营商套餐。 |
| 💰 **多货币** | 内置常用货币、中英文名称与符号；支持用户自行增删货币、设置手动汇率和基准货币。 |
| 💳 **付款方式** | 每位用户独立管理付款方式，可记录图标、账号和备注。 |
| 🔔 **通知提醒** | 支持 Telegram、飞书、QQ、Bark、邮件、Pushplus、Server 酱、企业微信、钉钉、Discord、Slack、ntfy、Gotify 和 Webhook 等渠道；支持到期提醒、免打扰时段和每周汇总。 |
| 🧪 **日期追踪** | 支持试用结束、取消截止日和付款卡有效期提醒。 |
| 📊 **仪表盘与报表** | 月度 / 年度支出、分类支出、排行、即将续费、过期订阅、一次性购买与 CSV 导出。 |
| 🗓️ **日历** | 提供日历视图和令牌保护的 `.ics` 订阅地址，可添加到 Apple、Google 或 Outlook 日历。 |
| 📚 **服务图标库** | 内置常见服务图标和官网；未缓存图标会由浏览器直接加载，后台只使用有界队列缓存，不会因远端图标不可访问而耗尽后端请求资源。 |
| 💾 **备份恢复** | 单用户 JSON 导入 / 导出、管理员整站备份与恢复、每日本地自动备份、WebDAV 备份 / 恢复及 CSV 导入 / 导出。 |
| 🔐 **安全与集成** | API Token、加密存储 WebDAV 与通知密钥、REST API、Streamable HTTP MCP 端点。 |
| 🌍 **体验** | 中文 / English / Русский，多主题，响应式 Web 界面和 PWA 前端资源。 |

> 本分支使用**内置 SQLite**，不需要 MySQL、数据库连接向导或网页初始化步骤。

---

## 🚀 快速开始

镜像由 GitHub Actions 在每次推送 `main` 后自动构建，并发布到 GitHub Container Registry：

```text
ghcr.io/chunyu-zhou/easysub:latest
```

### 方式 A：Docker Compose（推荐）

1. 下载 [`compose.yaml`](compose.yaml)。
2. 编辑其中的 `ADMIN_*` 和 `JWT_SECRET` 占位值。
3. 启动服务：

```bash
docker compose up -d
```

4. 浏览器访问：

```text
http://<服务器 IP>:8000
```

### 方式 B：docker run

```bash
docker run -d --name easysub \
  -p 8842:8000 \
  -e TZ=Asia/Shanghai \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD=replace-with-a-long-unique-password \
  -e ADMIN_EMAIL=admin@example.com \
  -e JWT_SECRET="$(openssl rand -hex 32)" \
  -v easysub_data:/app/data \
  --restart unless-stopped \
  ghcr.io/chunyu-zhou/easysub:latest
```

随后访问：

```text
http://<服务器 IP>:8842
```

### 首次启动行为

程序启动时会自动完成以下步骤：

1. 创建 `DATA_DIR`（默认 `data` 或容器内 `/app/data`）；
2. 创建或打开 SQLite 数据库；
3. 自动执行数据库迁移；
4. 写入内置分类、默认货币和默认付款方式；
5. 按 `ADMIN_USERNAME`、`ADMIN_PASSWORD`、`ADMIN_EMAIL` 创建首个管理员账号。

启动后直接使用管理员账号登录，**不会出现数据库安装或连接配置页面**。

---

## ⚙️ 配置说明

### 关键环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `ADMIN_USERNAME` | `admin` | 首次创建的管理员用户名。 |
| `ADMIN_PASSWORD` | `admin123` | 首次创建的管理员密码；生产环境必须修改。 |
| `ADMIN_EMAIL` | `admin@example.com` | 首次创建的管理员邮箱。 |
| `JWT_SECRET` | 自动生成并持久化 | 用于签发 JWT，并加密 WebDAV / 通知等敏感配置。生产环境推荐显式设置。 |
| `DATA_DIR` | `data` | SQLite 数据库、图标、备份和自动生成 JWT 密钥的持久化目录。 |
| `SQLITE_PATH` | `easysub.db` | SQLite 数据库文件名；相对路径位于 `DATA_DIR` 下，也可填写绝对路径。 |
| `LISTEN` | `:8000` | HTTP 监听地址。 |
| `TZ` | `Asia/Shanghai` | 时区，例如 `Asia/Shanghai`。 |
| `MCP_ENABLED` | `true` | 是否启用 `/mcp`。 |
| `REMINDER_SCAN_TIME` | `09:00` | 每日扫描续费和发送提醒的时间。 |
| `REQUIRE_ADMIN_APPROVAL` | `false` | 新注册用户是否需要管理员审核。 |
| `EXCHANGE_API_BASE` | `USD` | 汇率服务的系统基准货币。 |
| `EXCHANGE_API_URL` | `https://open.er-api.com/v6/latest/` | 汇率数据源。 |

> `ADMIN_*` 仅在数据库中尚不存在该管理员时生效。已经完成首次启动后，修改这些环境变量不会修改已有账号；请在网页 **设置 → 账户** 中修改密码和资料。

### 生成 `JWT_SECRET`

`JWT_SECRET` 必须是高随机性、私密且长期稳定的值。生成一次后，将**完整输出**填入 [`compose.yaml`](compose.yaml) 的 `JWT_SECRET`，并妥善保存到密码管理器或密钥管理系统。

Linux、macOS 或具备 OpenSSL 的 Docker 主机：

```bash
openssl rand -hex 32
```

Windows PowerShell：

```powershell
[Convert]::ToHexString((1..32 | ForEach-Object { Get-Random -Maximum 256 }))
```

两种命令都会生成 32 个随机字节编码后的 64 位十六进制字符串。不要使用文档示例、普通密码或已经提交到公开仓库的值。

**不要在正常升级时更换 `JWT_SECRET`。** 更换后，所有已登录会话将失效，已有的 WebDAV 和通知加密凭据也无法再解密。

如果未显式设置 `JWT_SECRET`，EasySub 会自动生成一个随机值并写入 `DATA_DIR/jwt_secret`。只要完整保留数据目录或 Docker 数据卷，这种方式也是安全的。

### SQLite 数据持久化

EasySub 默认使用：

```text
DATA_DIR/easysub.db
```

请持久化整个 `DATA_DIR`，其中包含：

- `easysub.db`、`easysub.db-wal`、`easysub.db-shm`；
- `jwt_secret`（未设置 `JWT_SECRET` 时）；
- 上传的图标；
- 本地自动备份；
- 运行所需的加密相关文件。

默认 [`compose.yaml`](compose.yaml) 使用 Docker 命名卷 `easysub_data`，推荐保持此方式。如果改用宿主机目录挂载，例如：

```yaml
volumes:
  - /volume1/docker/easysub/data:/app/data
```

该目录必须可被容器内的 `easysub` 用户写入。若日志出现 `unable to open database file` 或 `permission denied`，请修复宿主机目录权限后重建容器。

---

## 🔄 升级

数据存储在 `/app/data` 对应的持久化目录或卷中。升级时只更新镜像，不要删除该数据目录或卷。

使用 Compose：

```bash
docker compose pull && docker compose up -d --force-recreate
```

使用 `docker run`：

```bash
docker pull ghcr.io/chunyu-zhou/easysub:latest
```

然后以与原来相同的 `-v easysub_data:/app/data` 和环境变量重新创建容器。

> 升级前建议在网页 **设置 → 数据备份** 导出 JSON 备份。管理员还可以导出整站备份。

**不要执行以下命令，除非明确要删除全部数据：**

```bash
docker compose down -v
```

---

## 📖 使用要点

- **订阅与续费**：在“订阅”页面记录名称、金额、周期、下次续费日、分类、付款方式、网址和备注；可选择从今天或原到期日计算下一周期。
- **货币和付款方式**：在“设置”中维护自己的货币、中文名称、符号、手动汇率，以及带账号和备注的付款方式。
- **通知**：在“设置 → 通知”中配置渠道并执行测试；可设置提醒提前天数、免打扰时段和每周汇总。
- **WebDAV**：在“设置 → WebDAV”保存地址、用户名、密码和远程目录；可测试、备份、查看、删除和恢复远程 JSON 备份。
- **备份**：常规用户可以导出 / 导入自己的数据；管理员可执行整站备份和恢复。
- **日历**：在“设置 → 日历”获取 `.ics` 地址，添加到第三方日历应用。
- **API Token**：在“设置 → 安全”创建 API Token。令牌只会在创建时完整显示一次，请立即安全保存。

---

## 🔌 REST API 与 MCP

应用提供 REST API，API 基础路径为 `/api`。使用 API Token 或登录访问令牌进行认证：

```text
Authorization: Bearer <API_TOKEN>
```

常用接口包括：

| 分类 | 示例接口 |
| --- | --- |
| 订阅 | `GET/POST /api/subscriptions`、`PUT/DELETE /api/subscriptions/{id}`、`POST /api/subscriptions/{id}/renew` |
| 货币 | `/api/currencies`、`/api/currencies/catalog`、`/api/currencies/rate-table` |
| 付款方式 | `/api/payment-methods` |
| 报表 | `/api/dashboard`、`/api/reports/insights`、`/api/reports/export.csv` |
| 备份 | `/api/backup/export`、`/api/backup/import`、`/api/webdav/*` |
| 文档 | `/api/docs` |

MCP 使用 Streamable HTTP 端点：

```text
http://<服务器 IP>:8000/mcp
```

同样传入 `Authorization: Bearer <API_TOKEN>`。当前 MCP 提供只读的订阅列表、仪表盘和货币查询工具。

---

## ❓ 常见问题

<details>
<summary><b>提示 “embedded SQLite is unavailable” 或无法登录？</b></summary>

SQLite 无法创建、打开或迁移。先查看容器日志：

```bash
docker compose logs --tail=100 app
```

如果使用默认 Docker 命名卷，可安全重建应用容器（不会删除数据卷）：

```bash
docker compose pull && docker compose up -d --force-recreate
```

如果使用宿主机目录挂载，请确认该目录可被容器用户写入。不要删除 `/app/data` 或执行 `docker compose down -v`，除非确认要清空所有数据。
</details>

<details>
<summary><b>为什么修改 ADMIN_PASSWORD 后无法用新密码登录？</b></summary>

`ADMIN_*` 只用于首次创建管理员。数据库已有管理员后，环境变量不会覆盖账号密码。请使用原密码登录后在“设置 → 账户”中修改；如原密码遗失，应先备份数据，再通过受控方式重置数据库中的账号密码。
</details>

<details>
<summary><b>升级后会不会丢失 SQLite 数据？</b></summary>

只要保留 `/app/data` 对应的 Docker 卷或宿主机目录，升级只会自动执行兼容迁移，不会删除数据。升级前仍建议导出 JSON 备份。
</details>

<details>
<summary><b>支持 HTTPS 吗？</b></summary>

EasySub 直接提供 HTTP 服务。需要 HTTPS 时，可在应用前使用现有的 Nginx、Traefik、Cloudflare Tunnel、NAS 反向代理或其他 TLS 终止方案。项目不再内置 Caddy。
</details>

---

## 🧰 技术栈

| 组件 | 技术 |
| --- | --- |
| 后端 | Go · GORM · SQLite · cron |
| 前端 | Vue 3 · Vite · vue-i18n · Pinia |
| 数据库 | 内置 SQLite（纯 Go 驱动） |
| 部署 | Docker 多阶段构建 · GHCR 多架构镜像（linux/amd64、linux/arm64） |
| 数据安全 | JWT · bcrypt · 加密存储敏感配置 |

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request。提交前请运行：

```bash
go test ./...
```

```bash
cd frontend && npm run build
```

推送到 `main` 或创建 `v*` 标签后，GitHub Actions 会构建并发布 GHCR 多架构镜像。

---

## 📝 许可与致谢

本项目采用 [MIT License](./LICENSE)。

本项目基于 [suyijun8182/easysub](https://github.com/suyijun8182/easysub) 的 MIT 许可源码进行 Go / SQLite 改写；保留上游许可证，并感谢上游项目的设计与功能思路。
