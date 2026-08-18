# AGENTS.md

## 1. Project Overview（项目基础信息）
- 项目类型：企业内部服务器资产管理与工单流程系统，前后端分离、同仓分目录开发。
- 核心技术栈：后端 Go 1.26 + Gin + GORM + SQLite + JWT + LDAP；前端 Vue 3 + Vite 8 + Element Plus + Pinia。
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
- Go 单元测试与被测包同目录，文件名使用 `*_test.go`；跨包 HTTP/数据库集成测试统一放在 `backend/tests/`；e2e 专用配置和可执行夹具统一放在 `backend/tests/e2e/`，禁止放入 `cmd/` 或生产包。
- 前端单元测试统一放在 `frontend/tests/unit/`，Playwright 测试及启动夹具统一放在 `frontend/tests/e2e/`；生产源码目录 `frontend/src/` 不放测试文件。
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
- 遵循标准 Go 项目布局：
  - `cmd/<程序名>/` 只保留薄入口，负责日志、配置加载、信号和退出码，不承载业务初始化细节。
  - `internal/app/` 负责进程级依赖组装、HTTP 服务生命周期和后台任务启停。
  - `internal/httpapi/` 只负责协议适配、参数校验、认证授权和响应，不实现可复用业务规则。
  - `internal/service/` 放业务用例与领域服务，不得反向依赖 `httpapi`。
  - `internal/database/` 负责连接和版本化迁移；`internal/model/` 只放模型与业务枚举。
- 包依赖方向固定为 `cmd -> app -> httpapi/service/database/model`；禁止循环依赖，禁止用 `utils`、`common` 等无边界包堆放杂项。
- 包名使用简短、小写、单数名词；导出标识符必须有职责明确的 GoDoc。接口定义在消费方，只有存在替换实现或测试替身时才抽象接口。
- 长流程接收 `context.Context` 作为第一个参数并向下传递；错误使用 `%w` 保留错误链，HTTP 层统一转换为 `{"error":"..."}`，业务层不得直接写 HTTP 响应。
- HTTP 服务必须使用 `http.Server`，设置合理超时并处理 `SIGINT/SIGTERM`；关闭顺序为停止接收请求、等待在途请求、停止后台任务、关闭数据库。
- 新业务按领域拆文件，避免继续扩张综合型 `handler.go`、`models.go`；当一个文件承担多个不相关领域或明显难以审查时，先拆包内文件，再考虑抽包。
- 业务枚举集中放在 `backend/internal/model/types.go`，不要在业务逻辑中散落硬编码状态值。
- 工单状态流集中维护在 `backend/internal/service/ticket_flow.go`。
- 工单 PDF 归档通过 DOCX 模板填充后调用 LibreOffice 转换，模板位于 `templates/ticket-it-change-template.docx`。
- HTTP 层放在 `backend/internal/httpapi/`，认证、RBAC、AD 配置、资产、工单接口应保持 REST 风格。
- 数据模型放在 `backend/internal/model/`；数据库变更必须新增有序、可审计的版本化迁移，`AutoMigrate` 只能作为某个明确迁移步骤的实现，不得在启动流程中绕过迁移版本表直接调用。
- 错误返回使用统一 JSON 结构：`{"error": "..."}`。
- 避免变量名遮蔽导入包名，例如不要用 `config := ...` 遮蔽 `internal/config` 包。

### 4.2 API 与 Swagger
- 路由集中在 `internal/httpapi/router.go` 注册；处理函数按领域拆分文件，REST 路径、权限中间件和状态码必须可从路由表直接审查。
- 每个新增或变更的 `/api/v1` 接口必须同步 Swaggo 注解，至少包含 `Summary`、`Tags`、输入参数、成功响应、主要失败响应、`Security` 和 `Router`。
- Swagger 注解完成后必须重新生成 `backend/docs/docs.go`、`swagger.json`、`swagger.yaml`，禁止手工编辑生成文件。
- `/livez`、`/readyz`、`/metrics` 等不受 `/api/v1` BasePath 管理的运维端点记录在生产运维文档中，不得用错误 BasePath 强行写入 Swagger 2.0。

### 4.3 前端 Vue
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
- 提交必须包含更改内容摘要，不能只有一句提交标题；提交正文至少写明：
  - `Changes:` 本次修改的核心模块和行为变化。
  - `Validation:` 已执行的测试、构建或无法执行的原因。
  - 示例：
    ```text
    feat: add ticket archive downloads

    Changes:
    - add single and batch ticket archive download APIs
    - enforce participant permission checks for every archive

    Validation:
    - go test ./...
    - go build ./...
    - frontend Vite build
    ```
- 提交前检查：
  - 后端改动：`gofmt` + `go test ./...` + `go vet ./...` + `go build ./...`。
  - 前端改动：`npm run test:unit` + `npm run build`；涉及关键用户流程时增加或执行 Playwright。
  - 同时涉及前后端：后端测试与前端构建都要跑。
- 工作区可能存在用户未提交改动；提交前必须用 `git status --short` 确认范围，不要回退或夹带无关文件。

### 6.1 分支与合并
- 新功能必须在独立分支上开发，命名 `feature/<功能名>`，从最新的 main 分支切出：`git checkout main && git pull && git checkout -b feature/<功能名>`。
- **一个功能的所有开发与问题修复必须集中在同一个 `feature/<功能名>` 分支上完成**：功能合并前，测试缺陷、评审意见、需求调整一律直接提交到该功能分支；禁止为同一功能拆分多个散碎分支（例如 `feature/<功能名>-fix-1`、`feature/<功能名>-part-2`）。功能合并后发现的遗留问题，回到原功能分支修复（分支保留机制支持），修复完成后再次 squash merge 回 main。
- **功能分支必须按开发步骤分步提交**：模型/配置 → 核心逻辑 → 测试 → 前端 → 文档等各步骤独立 `git commit`（Conventional Commits），禁止把全部改动攒成一次提交就合并——功能分支保留的价值就是完整开发过程溯源，一次提交等于丢失开发过程。全部开发与验证完成后，才从 main 执行 squash merge。
- 结构重构也必须分步提交：先做无行为变化的移动/拆分并验证，再提交行为改动；禁止在同一提交中混入目录整理、依赖升级和业务功能。
- 禁止直接在 main 上提交功能代码；仅紧急修复（hotfix）可例外。
- 功能开发完整（后端测试/构建、前端构建全部通过）后，先更新文档再合并：
  - README.md：按现有 v2.x 章节风格增补功能说明。
  - Swagger：接口有变化时更新注解并重新生成 docs（`go run github.com/swaggo/swag/cmd/swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal`）。
  - 配置/部署：环境变量或部署方式变化时同步 `config.example.yaml` 与部署文档。
- 合并采用 squash merge：`git checkout main && git merge --squash feature/<功能名> && git commit`，提交信息遵循 Conventional Commits（含 Changes / Validation 正文），并在正文末尾标注 `branch: feature/<功能名>`，便于从 main 提交溯源到功能分支的完整开发过程（`git log --all --grep="branch: feature/<功能名>"` 或直接切到该分支查看）。
- 合并完成后**保留功能分支**，便于回溯功能开发过程与验证记录；不得删除功能分支（`git branch -D feature/<功能名>` 禁止使用）。如需归档，可将分支推送到远程长期保留。

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
