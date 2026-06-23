# 企业内部服务器资产管理系统

前后端分离的服务器资产台账与工单流程管理系统。

## 技术栈

- 后端：Go、Gin、GORM、SQLite、JWT
- 前端：Vue 3、Vite、Element Plus
- 部署：Docker Compose，前端 Nginx 反向代理 `/api/` 到后端

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

SQLite 数据通过 `backend-data` volume 持久化。
