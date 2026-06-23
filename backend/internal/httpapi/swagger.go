package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) SwaggerIndex(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>资产管理系统 API 文档</title>
  <style>
    body{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:32px;color:#1f2937}
    pre{background:#f5f7fb;border:1px solid #dbe2ea;border-radius:6px;padding:16px;overflow:auto}
    a{color:#2563eb}
  </style>
</head>
<body>
  <h1>资产管理系统 API 文档</h1>
  <p>OpenAPI JSON: <a href="/swagger/doc.json">/swagger/doc.json</a></p>
  <pre id="spec">Loading...</pre>
  <script>
    fetch('/swagger/doc.json').then(r=>r.json()).then(j=>{
      document.getElementById('spec').textContent = JSON.stringify(j, null, 2)
    })
  </script>
</body>
</html>`))
}

func (h *Handler) OpenAPISpec(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"openapi": "3.0.3",
		"info": gin.H{
			"title":   "Asset Registration Management System API",
			"version": "1.0.0",
		},
		"servers": []gin.H{{"url": "/api/v1"}},
		"paths": gin.H{
			"/auth/login":            gin.H{"post": gin.H{"summary": "用户登录"}},
			"/auth/logout":           gin.H{"post": gin.H{"summary": "用户退出"}},
			"/auth/me":               gin.H{"get": gin.H{"summary": "当前用户"}},
			"/roles":                 gin.H{"get": gin.H{"summary": "角色列表"}},
			"/users":                 gin.H{"get": gin.H{"summary": "用户列表"}, "post": gin.H{"summary": "创建用户"}},
			"/users/{id}":            gin.H{"put": gin.H{"summary": "更新用户"}},
			"/assets":                gin.H{"get": gin.H{"summary": "资产列表"}, "post": gin.H{"summary": "创建资产"}},
			"/assets/{id}":           gin.H{"get": gin.H{"summary": "资产详情"}, "put": gin.H{"summary": "更新资产"}, "delete": gin.H{"summary": "删除资产"}},
			"/tickets":               gin.H{"get": gin.H{"summary": "工单列表"}, "post": gin.H{"summary": "创建工单"}},
			"/tickets/{id}":          gin.H{"get": gin.H{"summary": "工单详情"}, "put": gin.H{"summary": "更新工单"}},
			"/tickets/{id}/submit":   gin.H{"post": gin.H{"summary": "提交工单"}},
			"/tickets/{id}/approve":  gin.H{"post": gin.H{"summary": "审批通过"}},
			"/tickets/{id}/reject":   gin.H{"post": gin.H{"summary": "审批驳回"}},
			"/tickets/{id}/start":    gin.H{"post": gin.H{"summary": "开始执行"}},
			"/tickets/{id}/complete": gin.H{"post": gin.H{"summary": "执行完成"}},
			"/tickets/{id}/close":    gin.H{"post": gin.H{"summary": "关闭工单"}},
			"/tickets/{id}/cancel":   gin.H{"post": gin.H{"summary": "取消工单"}},
		},
	})
}
