# 企业内部服务器资产管理系统

面向企业内部运维场景的服务器资产台账与工单流程系统，采用前后端分离架构，覆盖资产登记、自动发现、审批执行、盘点、机柜、凭据、软件许可和生产运维。

## 项目概览

| 项目 | 技术或说明 |
| --- | --- |
| 后端 | Go 1.26、Gin、GORM、SQLite、JWT、LDAP |
| 前端 | Vue 3、Vite 8、Element Plus、Pinia |
| API | REST API、Swaggo / Swagger |
| 数据存储 | SQLite 和后端本地文件目录 |
| 身份认证 | 本地账号与 AD/LDAP 混合认证 |
| 部署方式 | 单实例 Go 服务、Nginx、systemd |

后端仅提供 REST API，不嵌入或托管前端静态文件。前端位于 `frontend/`，后端位于 `backend/`。

## 文档

- [用户手册](docs/USER-MANUAL.md)：登录、角色权限、资产、工单、管理员配置和常见问题。
- [生产就绪说明](docs/PRODUCTION-READINESS.md)：健康检查、备份恢复、发布验证和故障处理。
- [Ubuntu 26 部署手册](deploy/ubuntu26/README-DEPLOY.md)：Nginx、systemd 和生产配置。
- [更新记录](docs/CHANGELOG.md)：按版本倒序整理的功能变化。
- [开发规范](AGENTS.md)：Go 规范、项目架构、测试和提交要求。

## 核心功能

### 资产管理

- 服务器资产新增、编辑、批量修改、删除和退役归档。
- CSV / XLSX 导入、CSV 导出、统计报表和二维码标签打印。
- 资产生命周期、在线状态、维保到期、机房机柜和多 IP 管理。
- 按角色限制资产数据范围，保留字段级快照和操作审计。

### 自动发现

- 基于 nmap 的单 IP、IP 列表和 CIDR 网段扫描。
- 探活与详扫两阶段执行，支持分片并行、扫描时段和增量扫描。
- MAC、多 IP 匹配以及新增、变更、恢复在线和离线识别。
- 低风险自动应用、高风险人工确认、变化告警和自动建单。

### 工单流程

- 资产登记、资产变更、资产退役、维护、巡检和许可证续费工单。
- 可配置多节点审批、代理审批、审批转交和批量审批。
- 草稿、审批、执行、验收、关闭和 PDF 归档完整流程。
- SLA 时限、邮件和 IM 通知、评论、附件、统计和批量归档下载。

### 运维管理

- 资产盘点、机房机柜、IP 地址池、凭据保险库和软件许可。
- AD 用户导入、本地用户、角色和代理审批人管理。
- 加密完整备份、恢复校验、异地副本和任务运行中心。
- `/livez`、`/readyz`、`/metrics`、结构化日志和请求 ID。

## 环境要求

### 开发环境

- Go 1.26
- Node.js 22
- npm

### 可选运行依赖

- nmap：资产自动发现。
- LibreOffice Writer：工单 DOCX 模板转 PDF。
- Nginx：生产环境前端托管和 API 反向代理。

Windows 可使用脚本安装便携版 nmap：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/setup-nmap.ps1
```

Linux 或 macOS：

```bash
bash scripts/setup-nmap.sh
```

## 快速开始

### 1. 配置后端

进入后端目录并复制配置样例：

```bash
cd backend
cp config.example.yaml config.yaml
```

Windows PowerShell：

```powershell
cd backend
Copy-Item config.example.yaml config.yaml
```

开发环境可以使用样例默认值。生产环境必须修改：

```yaml
app:
  env: production

security:
  jwt_secret: replace-with-a-strong-random-secret
  config_encryption_key: replace-with-a-strong-random-key

admin:
  password: replace-with-a-strong-admin-password
```

### 2. 启动后端

```bash
cd backend
go mod tidy
go run ./cmd/server
```

### 3. 启动前端

在另一个终端执行：

```bash
cd frontend
npm install
npm run dev
```

### 4. 访问系统

| 服务 | 默认地址 |
| --- | --- |
| 前端 | <http://localhost:5173> |
| 后端健康检查 | <http://localhost:8080/healthz> |
| 存活检查 | <http://localhost:8080/livez> |
| 就绪检查 | <http://localhost:8080/readyz> |
| Swagger | <http://localhost:8080/swagger/index.html> |

Swagger 仅在 `swagger.enabled: true` 时开放。

开发环境默认管理员为 `admin / admin123456`。首次登录必须修改密码，生产环境禁止继续使用默认密码。

## 配置

后端默认读取 `backend/config.yaml`。也可以通过 `CONFIG_FILE` 指定其他 YAML 文件：

```bash
CONFIG_FILE=/path/to/config.yaml go run ./cmd/server
```

完整配置项参见 [backend/config.example.yaml](backend/config.example.yaml)。主要配置分组：

| 分组 | 用途 |
| --- | --- |
| `app` | 运行环境 |
| `http` | 后端监听地址 |
| `storage` | 数据库、附件、归档和备份目录 |
| `security` | JWT 和敏感配置加密密钥 |
| `auth` | 本地、AD 或混合认证模式 |
| `swagger` | Swagger 开关 |
| `admin` | 初始管理员 |
| `cors` | 前端来源限制 |
| `discovery` | nmap、扫描并发、目标上限和告警 |

前端 API 地址通过 `VITE_API_BASE_URL` 配置，默认值为 `/api/v1`。

## 项目结构

```text
.
├── backend/
│   ├── cmd/server/          # 服务入口
│   ├── internal/app/        # 依赖组装和生命周期
│   ├── internal/config/     # 配置
│   ├── internal/database/   # 数据库和迁移
│   ├── internal/httpapi/    # HTTP、认证、权限和 Swagger 注解
│   ├── internal/model/      # 模型和业务枚举
│   ├── internal/service/    # 业务服务和后台任务
│   ├── tests/               # 跨包 HTTP / 数据库集成测试
│   └── docs/                # 生成的 Swagger 文档
├── frontend/
│   ├── src/                 # Vue 应用
│   └── tests/               # Vitest 和 Playwright 测试
├── deploy/ubuntu26/         # Ubuntu 部署文件
├── docs/                    # 用户、生产和版本文档
├── scripts/                 # 安装与检查脚本
├── templates/               # 导入和工单归档模板
├── AGENTS.md                # 开发规范
└── README.md
```

## 开发与验证

### 后端

```bash
cd backend
gofmt -w ./cmd ./internal ./tests ./tools
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

### 前端

```bash
cd frontend
npm run test:unit
npm run build
npm run test:e2e
```

CI 会在 `main` 和所有 `feature/**` 分支执行格式、模块一致性、测试、race、vet、构建、依赖审计和端到端测试。

## Swagger

接口注解位于 `backend/internal/httpapi/`。接口变化后在 `backend/` 目录重新生成文档：

```bash
go run github.com/swaggo/swag/cmd/swag init \
  -g cmd/server/main.go \
  -o docs \
  --parseDependency \
  --parseInternal
```

生成文件：

- `backend/docs/docs.go`
- `backend/docs/swagger.json`
- `backend/docs/swagger.yaml`

不要手工编辑生成文件。

## 部署

生产部署请按以下顺序阅读：

1. [Ubuntu 26 部署手册](deploy/ubuntu26/README-DEPLOY.md)
2. [生产就绪说明](docs/PRODUCTION-READINESS.md)
3. [用户手册中的备份恢复章节](docs/USER-MANUAL.md#128-数据备份)

生产环境至少应完成：

- 修改默认管理员密码、JWT 密钥和配置加密密钥。
- 限制 CORS、Swagger、指标和管理端点的网络访问。
- 配置可写的数据目录及最小化运行权限。
- 创建并校验完整备份，保留异地副本。
- 验证 `/livez`、`/readyz`、登录、资产、工单和备份。

## 数据目录

默认本地数据路径：

| 数据 | 默认路径 |
| --- | --- |
| SQLite | `backend/data/assets.db` |
| 附件 | `backend/data/attachments/` |
| 工单归档 | `backend/data/ticket-archives/` |
| 完整备份 | `backend/data/backups/` |

这些目录不应提交到 Git。升级、迁移或清理前必须先创建并验证备份。

## 安全说明

- 不得在代码、配置样例、日志或工单中写入真实密钥。
- AD Bind 密码、凭据和许可证密钥使用配置加密密钥保护。
- 敏感明文查看和备份下载需要二次认证并记录审计。
- 工单附件和归档仅允许参与人访问。
- 生产环境应关闭或限制 Swagger 和 `/metrics`。

## 更新记录

当前稳定能力以 v2.22 为基线。详细版本变化见 [更新记录](docs/CHANGELOG.md)。
