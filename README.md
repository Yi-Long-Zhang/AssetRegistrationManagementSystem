# 企业内部服务器资产管理系统

前后端分离的服务器资产台账与工单流程管理系统。

## 技术栈

- 后端：Go、Gin、GORM、SQLite、JWT
- 前端：Vue 3、Vite、Element Plus
- 运行方式：本地分别启动后端 API 与前端 Vite 开发服务
- 附件：后端本地目录存储，默认目录 `backend/data/attachments`
- 工单归档：关闭后生成 PDF，默认目录 `backend/data/ticket-archives`
- 用户体系：支持本地账号和 AD/LDAP 账号混合登录

## 本地开发

后端：

```bash
cd backend
go mod tidy
go run ./cmd/server
```

前端：

```bash
cd frontend
npm install
npm run dev
```

默认访问地址：

- 前端：http://localhost:5173
- 后端健康检查：http://localhost:8080/healthz
- API 文档：在配置文件中设置 `swagger.enabled: true` 后访问 http://localhost:8080/swagger/index.html

后端配置由 YAML 文件决定。默认尝试读取 `backend/config.yaml`，如不存在则使用开发默认值；也可以用 `CONFIG_FILE` 指定配置文件路径：

```text
CONFIG_FILE=./config.yaml
```

可从 `backend/config.example.yaml` 复制本地配置。

Swagger 文档通过 `swaggo` 注解生成，更新接口注解后在 `backend/` 下运行：

```bash
go run github.com/swaggo/swag/cmd/swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
```

默认管理员账号：

```text
admin / admin123456
```

## 资产导入导出

- 管理员可在“服务器资产”页面新增、编辑、删除资产。
- 资产导入支持 CSV / XLSX：先下载 Excel 导入模板，按测绘结果表头填写后上传。
- 导入模板字段按“全资产详细清单”设计：序号、IP地址、主机名/设备名称、MAC地址、厂商、资产类型、操作系统、开放端口、运行服务/应用、应用版本、资产归属/负责人、所在网段、备注。
- 导入时系统根据“IP地址”自动生成内部资产编号：编号不存在则新增，编号已存在则更新。
- 批量导出使用 CSV，导出字段与导入模板一致。
- 导入模板和导出文件不包含“风险等级”字段。
- 仓库内提供可直接使用的模板文件：`templates/asset-import-template.xlsx`。

## v2.4 工单流程与归档

- 管理员可在“流程配置”页面按工单类型维护审批流程。
- 审批节点支持排序，每个节点可指定多个审批人，任一审批人通过即可进入下一节点。
- 提交工单后，如已启用邮件通知，系统会自动通知当前审批节点的审批人。
- 工单统一流转：草稿、审批中、已驳回、已审批、执行中、待验收、已关闭、已取消。
- 申请人验收通过后，系统写回资产台账并生成《IT配置变更申请表》PDF 归档。
- 工单关闭后可在详情页下载单个归档 PDF，也可在列表勾选多个已归档工单批量下载 ZIP。
- 工单列表支持“我的待办 / 我提交的 / 全部”视图。
- 工单详情支持参与人评论、附件上传和下载。

归档相关配置位于 YAML 的 `storage` 节点：

```yaml
storage:
  ticket_archive_dir: data/ticket-archives
  ticket_template_path: ../templates/ticket-it-change-template.docx
  libreoffice_bin: soffice
```

生成 PDF 需要本机安装 LibreOffice。Ubuntu 部署建议安装 `libreoffice-writer` 和 `fonts-noto-cjk`。

## 系统配置

- 管理员可在“系统配置”页面维护 AD 域控和 SMTP 邮件通知配置。
- SMTP 密码会用配置文件中的 `security.config_encryption_key` 加密后保存。

## v2.2 AD 域控接入

- 管理员可在“系统配置”页面维护单域 AD 配置。
- AD 配置支持普通 LDAP，例如 `ldap://dc.example.com:389`。
- 页面默认用“登录名格式 / 只查用户 / 排除禁用账号”生成 LDAP 过滤器，高级过滤器默认隐藏。
- Bind 密码会用配置文件中的 `security.config_encryption_key` 加密后保存。
- 管理员可按 `sAMAccountName` 查询 AD 用户，并导入为本系统用户。
- AD 只负责认证，系统角色仍在本系统分配。

相关配置：

```yaml
auth:
  mode: mixed
security:
  config_encryption_key: change-me-config-key
swagger:
  enabled: false
```
