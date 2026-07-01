# AGENTS.md

## 1. Project Overview（项目基础信息）
- 项目类型：企业内部服务器资产管理与工单流程系统，前后端分离、同仓分目录开发。
- 核心技术栈：后端 Go 1.26 + Gin + GORM + SQLite + JWT + LDAP；前端 Vue 3 + Vite 5 + Element Plus + Pinia。
- 项目用途：管理内部服务器资产台账、工单审批执行、用户角色、AD/LDAP 用户导入与混合登录。
- 核心约束：后端只提供 REST API，不内嵌或托管前端静态资源；前端独立放在 `frontend/`；SQLite 和附件使用后端本地目录；系统角色由本系统维护，AD 只负责认证。

## 2. Setup & Development（环境搭建与开发流程）

### 2.1 后端开发
- 目录：`backend/`
- 安装/同步依赖：`go mod tidy`
- 本地运行：`go run ./cmd/server`
- 构建检查：`go build ./...`
- 默认管理员：`admin / admin123456`
- 关键环境变量：
  - `HTTP_ADDR=:8080`
  - `DATABASE_PATH=./data/assets.db`
  - `ATTACHMENT_DIR=./data/attachments`
  - `TICKET_ARCHIVE_DIR=./data/ticket-archives`
  - `TICKET_TEMPLATE_PATH=../templates/ticket-it-change-template.docx`
  - `LIBREOFFICE_BIN=soffice`
  - `JWT_SECRET=change-me-in-production`
  - `AUTH_MODE=mixed`
  - `CONFIG_ENCRYPTION_KEY=change-me-config-key`

### 2.2 前端开发
- 目录：`frontend/`
- 包管理器：npm（当前仓库使用 `package-lock.json`）
- 安装依赖：`npm install`
- 本地运行：`npm run dev`
- 构建：`npm run build`
- API 地址：通过 `VITE_API_BASE_URL` 配置，默认 `/api/v1`。

### 2.3 本地访问
- 前端访问：http://localhost:5173
- 后端健康检查：http://localhost:8080/healthz
- API 文档：http://localhost:8080/swagger/index.html
- 本地持久化：SQLite 默认使用 `backend/data/assets.db`，附件默认使用 `backend/data/attachments`，工单归档 PDF 默认使用 `backend/data/ticket-archives`。

## 3. Testing Guidelines（测试规范）
- 后端测试框架：Go testing。
- 后端测试命令：
  - 常规：`go test ./...`
  - Windows/受限环境推荐：
    `set TMP=%CD%\\.tmp&& set TEMP=%CD%\\.tmp&& set GOCACHE=%CD%\\.gocache&& go test ./...`
- 后端静态检查：`go vet ./...`
- 后端构建检查：`go build ./...`
- 前端构建检查：`npm run build`
- 修改后必须至少运行与改动相关的验证：
  - 后端业务、模型、认证、权限、工单流：运行 `go test ./...`。
  - 前端路由、状态、API、组件、字典：运行 `npm run build`。
  - 依赖升级：同时运行 `go test ./...`、`go build ./...`、`go vet ./...` 和 `npm run build`（如影响前端）。

## 4. Code Style（代码风格规范）

### 4.1 后端 Go
- 使用 `gofmt` 格式化 Go 文件。
- 业务枚举集中放在 `backend/internal/model/types.go`，不要在业务逻辑中散落硬编码状态值。
- 工单状态流集中维护在 `backend/internal/service/ticket_flow.go`。
- 工单 PDF 归档通过 DOCX 模板填充后调用 LibreOffice 转换，模板位于 `templates/ticket-it-change-template.docx`。
- HTTP 层放在 `backend/internal/httpapi/`，认证、RBAC、AD 配置、资产、工单接口应保持 REST 风格。
- 数据模型放在 `backend/internal/model/`，数据库迁移由 GORM AutoMigrate 统一处理。
- 错误返回使用统一 JSON 结构：`{"error": "..."}`。
- 避免变量名遮蔽导入包名，例如不要用 `config := ...` 遮蔽 `internal/config` 包。

### 4.2 前端 Vue
- 使用 Vue 3 Composition API 和 `<script setup>`。
- API 调用按业务模块放在 `frontend/src/api/`，页面不要直接重复创建 Axios 实例。
- 登录态、用户信息、页面状态放在 Pinia store：`frontend/src/stores/`。
- 路由权限使用 `router.js` 的 `meta.roles`、`meta.menu`、`meta.title`、`meta.icon`。
- 业务字典集中放在 `frontend/src/constants/dictionaries.js`，前后端状态值必须一致，例如取消状态为 `cancelled`。
- 通用 UI 组件放在 `frontend/src/components/common/`；页面保持业务编排，不堆重复基础组件逻辑。
- 资产导入支持 CSV / XLSX，默认下载 Excel 模板，批量导出使用 CSV；模板、导入、导出字段必须按测绘表头保持一致：序号、IP地址、主机名/设备名称、MAC地址、厂商、资产类型、操作系统、开放端口、运行服务/应用、应用版本、资产归属/负责人、所在网段、备注，且不得加入“风险等级”字段。

## 5. 操作边界与禁止行为

### 5.1 禁止操作
- 不得把前端页面写入 Go 文件、Go template 或嵌入后端二进制。
- 不得提交 `node_modules/`、`dist/`、`backend/data/`、`.gocache/`、`.tmp/`、IDE 配置目录。
- 不得硬编码真实密钥、真实域控地址、真实账号密码。
- 不得删除历史业务模型、工单状态、审计记录相关逻辑，除非有明确迁移方案。
- 不得随意修改 `backend/data/` 下 SQLite 和附件的默认持久化路径。

### 5.2 高风险改动
- 修改认证、JWT、RBAC、AD/LDAP、密码加密、附件下载权限前，必须先说明影响范围和回归测试点。
- 升级依赖前先查看现有 `go.mod`、`package-lock.json`，避免引入重复库或破坏锁定版本。
- 修改工单状态值时，必须同步后端枚举、状态机、前端字典、测试用例。
- 修改工单流程时，必须同步流程配置接口、待办筛选、详情页流程进度、归档 PDF 内容。
- 修改数据库模型时，必须确认 SQLite AutoMigrate 的兼容性和已有数据影响。

## 6. Commit & Review（提交与协作）
- 提交信息建议使用英文 Conventional Commits：
  - `feat: add ticket attachments`
  - `fix: align cancelled ticket status`
  - `chore: update backend dependencies`
- 提交前检查：
  - 后端改动：`gofmt` + `go test ./...`，必要时加 `go build ./...`、`go vet ./...`。
  - 前端改动：`npm run build`。
  - 同时涉及前后端：后端测试与前端构建都要跑。
- 工作区可能存在用户未提交改动；提交前必须用 `git status --short` 确认范围，不要回退或夹带无关文件。

## 7. Security Notes（安全规范）
- `JWT_SECRET`、`CONFIG_ENCRYPTION_KEY`、AD Bind 密码不得写入代码或文档示例之外的真实值。
- AD Bind 密码必须通过 AES-GCM 加密后入库，密钥来自 `CONFIG_ENCRYPTION_KEY`。
- AD 用户必须先由管理员导入系统后才能登录；AD 只做认证，不自动授予系统角色。
- 工单评论、附件、下载接口必须限制为参与人可见：申请人、审批人、执行人、资产管理员或管理员。
- 工单归档 PDF 只能在已关闭状态下载；单个下载和批量 ZIP 下载都必须校验每个工单的参与人权限，不能静默跳过越权工单。
- 所有资产关键变更和工单状态变化应写入审计/时间线记录。

## 8. Debugging Notes（调试与排错）
- Windows 下 Go 测试如果出现临时 exe 被拦截或 SQLite 文件被占用，优先使用项目内 `TMP/TEMP/GOCACHE`，并确认测试关闭数据库连接。
- 前端构建出现大 chunk 警告时不代表构建失败；后续可通过路由懒加载或 Rollup manualChunks 优化。
- AD 连接测试失败时，优先检查 LDAP URL、Base DN、Bind DN、Bind 密码和用户过滤器生成结果。
- 若 GoLand/Mend 报依赖 CVE，优先升级 `go.mod` 中直接依赖，再运行完整后端验证。
