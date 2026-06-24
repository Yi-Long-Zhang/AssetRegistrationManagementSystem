# 企业内部服务器资产管理系统

前后端分离的服务器资产台账与工单流程管理系统。

## 技术栈

- 后端：Go、Gin、GORM、SQLite、JWT
- 前端：Vue 3、Vite、Element Plus
- 运行方式：本地分别启动后端 API 与前端 Vite 开发服务
- 附件：后端本地目录存储，默认目录 `backend/data/attachments`
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
- API 文档：http://localhost:8080/swagger/index.html

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

## v2.1 工单增强

- 管理员可配置每种工单类型的默认审批人。
- 提交工单时系统自动绑定默认审批人，未配置时会阻止提交。
- 工单列表支持“我的待办 / 我提交的 / 全部”视图。
- 工单详情支持参与人评论、附件上传和下载。

## v2.2 AD 域控接入

- 管理员可在用户角色页维护单域 AD 配置。
- AD 配置支持普通 LDAP，例如 `ldap://dc.example.com:389`。
- 页面默认用“登录名格式 / 只查用户 / 排除禁用账号”生成 LDAP 过滤器，高级过滤器默认隐藏。
- Bind 密码会用 `CONFIG_ENCRYPTION_KEY` 加密后保存。
- 管理员可按 `sAMAccountName` 查询 AD 用户，并导入为本系统用户。
- AD 只负责认证，系统角色仍在本系统分配。

相关环境变量：

```text
AUTH_MODE=mixed
CONFIG_ENCRYPTION_KEY=change-me-config-key
```
