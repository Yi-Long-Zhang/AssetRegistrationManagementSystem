# 企业内部服务器资产管理系统 Ubuntu 26 部署手册

本文档适用于 Ubuntu 26.x amd64 服务器，采用 systemd 运行 Go 后端，Nginx 托管 Vue 前端并反向代理 `/api/`、`/swagger/`、`/healthz` 到后端。

## 1. 部署目录

默认安装到：

```bash
/opt/asset-management
```

安装后目录结构：

```text
/opt/asset-management/
├── backend/asset-management-server
├── frontend/
├── data/assets.db
├── data/attachments/
├── backups/
└── .env
```

## 2. 服务器依赖

安装脚本会自动安装：

```bash
sudo apt update
sudo apt install -y nginx ca-certificates tar rsync
```

如服务器不能访问外网，请提前用离线方式安装 Nginx。

## 3. 上传部署包

将 `asset-management-ubuntu26.zip` 上传到服务器，例如：

```bash
scp asset-management-ubuntu26.zip user@SERVER_IP:/tmp/
```

服务器上解压：

```bash
cd /tmp
unzip asset-management-ubuntu26.zip
cd asset-management-ubuntu26
```

## 4. 修改配置

复制并编辑环境变量：

```bash
cp config/asset-management.env.example config/asset-management.env
nano config/asset-management.env
```

必须修改：

```text
JWT_SECRET=请改成高强度随机字符串
CONFIG_ENCRYPTION_KEY=请改成高强度随机字符串
ADMIN_PASSWORD=请改成强密码
```

可选修改：

```text
HTTP_ADDR=127.0.0.1:8080
DATABASE_PATH=/opt/asset-management/data/assets.db
ATTACHMENT_DIR=/opt/asset-management/data/attachments
AUTH_MODE=mixed
```

生成随机密钥示例：

```bash
openssl rand -base64 32
```

## 5. 安装

执行：

```bash
sudo bash install.sh
```

安装脚本会：

- 创建系统用户 `assetmgmt`
- 安装后端二进制和前端静态文件
- 写入 `/opt/asset-management/.env`
- 创建 SQLite 与附件目录
- 安装 systemd 服务
- 安装 Nginx 站点配置
- 启动服务

## 6. 访问

浏览器访问：

```text
http://服务器IP/
```

健康检查：

```bash
curl http://127.0.0.1:8080/healthz
curl http://服务器IP/healthz
```

默认管理员：

```text
admin / 你在 config/asset-management.env 中设置的 ADMIN_PASSWORD
```

## 7. 迁移现有数据库

如果要把开发机已有数据迁移到服务器：

1. 停服务：

```bash
sudo systemctl stop asset-management
```

2. 备份服务器当前数据库：

```bash
sudo cp /opt/asset-management/data/assets.db /opt/asset-management/backups/assets.db.$(date +%Y%m%d%H%M%S)
```

3. 上传开发机数据库到服务器：

```bash
scp backend/data/assets.db user@SERVER_IP:/tmp/assets.db
```

4. 替换数据库并授权：

```bash
sudo cp /tmp/assets.db /opt/asset-management/data/assets.db
sudo chown assetmgmt:assetmgmt /opt/asset-management/data/assets.db
sudo chmod 640 /opt/asset-management/data/assets.db
```

5. 启动服务：

```bash
sudo systemctl start asset-management
```

## 8. 日常运维

查看服务状态：

```bash
sudo systemctl status asset-management
```

查看后端日志：

```bash
sudo journalctl -u asset-management -f
```

重启服务：

```bash
sudo systemctl restart asset-management
sudo systemctl reload nginx
```

备份数据：

```bash
sudo bash /opt/asset-management/scripts/backup.sh
```

备份文件默认输出到：

```text
/opt/asset-management/backups/
```

## 9. 升级

上传新部署包后执行：

```bash
cd /tmp/asset-management-ubuntu26
sudo bash install.sh
```

脚本会保留 `/opt/asset-management/data` 和 `/opt/asset-management/.env`，覆盖后端二进制和前端静态文件。

## 10. 常见问题

- 页面能打开但接口失败：检查 Nginx 配置是否启用，执行 `sudo nginx -t && sudo systemctl reload nginx`。
- 后端启动失败：执行 `sudo journalctl -u asset-management -n 100 --no-pager` 查看错误。
- 数据库无权限：确认 `/opt/asset-management/data` 属主为 `assetmgmt:assetmgmt`。
- AD 配置保存失败：确认 `CONFIG_ENCRYPTION_KEY` 已设置且保持不变，修改密钥会导致已保存的 Bind 密码无法解密。
