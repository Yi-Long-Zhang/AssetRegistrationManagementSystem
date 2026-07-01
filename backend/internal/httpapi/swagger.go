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
			"/auth/login":                   gin.H{"post": gin.H{"summary": "用户登录"}},
			"/auth/logout":                  gin.H{"post": gin.H{"summary": "用户退出"}},
			"/auth/me":                      gin.H{"get": gin.H{"summary": "当前用户"}},
			"/ad/config":                    gin.H{"get": gin.H{"summary": "AD 配置详情"}, "put": gin.H{"summary": "保存 AD 配置"}},
			"/ad/test":                      gin.H{"post": gin.H{"summary": "测试 AD 连接"}},
			"/ad/lookup-user":               gin.H{"post": gin.H{"summary": "查询 AD 用户"}},
			"/ad/import-user":               gin.H{"post": gin.H{"summary": "导入 AD 用户"}},
			"/settings/mail":                gin.H{"get": gin.H{"summary": "邮件配置详情"}, "put": gin.H{"summary": "保存邮件配置"}},
			"/settings/mail/test":           gin.H{"post": gin.H{"summary": "测试邮件发送"}},
			"/roles":                        gin.H{"get": gin.H{"summary": "角色列表"}},
			"/ticket-type-approvers":        gin.H{"get": gin.H{"summary": "工单类型审批配置列表"}},
			"/ticket-type-approvers/{type}": gin.H{"put": gin.H{"summary": "设置工单类型默认审批人"}},
			"/workflows":                    gin.H{"get": gin.H{"summary": "流程配置列表"}},
			"/workflows/{type}":             gin.H{"get": gin.H{"summary": "工单类型流程配置"}, "put": gin.H{"summary": "保存工单类型流程配置"}},
			"/workflows/{type}/enable":      gin.H{"post": gin.H{"summary": "启用工单类型流程"}},
			"/users":                        gin.H{"get": gin.H{"summary": "用户列表"}, "post": gin.H{"summary": "创建用户"}},
			"/users/{id}":                   gin.H{"put": gin.H{"summary": "更新用户"}},
			"/assets":                       gin.H{"get": gin.H{"summary": "资产列表（分页/筛选/排序）"}, "post": gin.H{"summary": "创建资产"}},
			"/assets/stats":                 gin.H{"get": gin.H{"summary": "资产统计概览"}},
			"/assets/export":                gin.H{"get": gin.H{"summary": "批量导出资产 CSV"}},
			"/assets/template":              gin.H{"get": gin.H{"summary": "下载资产导入模板"}},
			"/assets/import":                gin.H{"post": gin.H{"summary": "批量导入资产 CSV"}},
			"/assets/{id}":                  gin.H{"get": gin.H{"summary": "资产详情"}, "put": gin.H{"summary": "更新资产"}, "delete": gin.H{"summary": "删除资产"}},
			"/tickets":                      gin.H{"get": gin.H{"summary": "工单列表"}, "post": gin.H{"summary": "创建工单"}},
			"/tickets/archives/download":    gin.H{"post": gin.H{"summary": "批量下载工单归档 PDF ZIP"}},
			"/tickets/{id}":                 gin.H{"get": gin.H{"summary": "工单详情"}, "put": gin.H{"summary": "更新工单"}},
			"/tickets/{id}/comments":        gin.H{"get": gin.H{"summary": "工单评论列表"}, "post": gin.H{"summary": "创建工单评论"}},
			"/tickets/{id}/attachments":     gin.H{"get": gin.H{"summary": "工单附件列表"}, "post": gin.H{"summary": "上传工单附件"}},
			"/tickets/{id}/attachments/{attachmentId}/download": gin.H{"get": gin.H{"summary": "下载工单附件"}},
			"/tickets/{id}/archive/download":                    gin.H{"get": gin.H{"summary": "下载工单归档 PDF"}},
			"/tickets/{id}/submit":                              gin.H{"post": gin.H{"summary": "提交工单"}},
			"/tickets/{id}/approve":                             gin.H{"post": gin.H{"summary": "审批通过"}},
			"/tickets/{id}/reject":                              gin.H{"post": gin.H{"summary": "审批驳回"}},
			"/tickets/{id}/start":                               gin.H{"post": gin.H{"summary": "开始执行"}},
			"/tickets/{id}/complete":                            gin.H{"post": gin.H{"summary": "执行完成"}},
			"/tickets/{id}/accept":                              gin.H{"post": gin.H{"summary": "验收通过并归档"}},
			"/tickets/{id}/cancel":                              gin.H{"post": gin.H{"summary": "取消工单"}},
		},
	})
}
