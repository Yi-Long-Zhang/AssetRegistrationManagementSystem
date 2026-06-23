# 企业内部服务器资产管理系统

前后端分离的服务器资产台账与工单流程管理系统。

## 技术栈

- 后端：Go、Gin、GORM、SQLite、JWT
- 前端：Vue 3、Vite、Element Plus
- 部署：Docker Compose，前端 Nginx 反向代理 `/api/` 到后端
- 附件：后端本地 volume 持久化，默认目录 `data/attachments`
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

默认管理员账号：

```text
admin / admin123456
```

## Docker Compose

```bash
docker compose up --build
```

- 前端：http://localhost:8081
- 后端健康检查：http://localhost:8080/healthz
- API 文档：http://localhost:8080/swagger/index.html

SQLite 数据和工单附件通过 `backend-data` volume 持久化。

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
