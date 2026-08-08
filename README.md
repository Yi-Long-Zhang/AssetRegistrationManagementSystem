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

## v2.5 动态资产管理（自动发现 + 变更追踪）

- 管理员或资产管理员可在“资产发现”页面维护发现规则：扫描目标（IP/CIDR）、端口、服务识别（`-sV`）、调度间隔、是否自动纳管。
- 发现引擎调用系统 **nmap** 执行扫描，解析 `-oX` XML 输出；nmap 通过 `discovery.nmap_bin` 配置或自动探测便携目录/系统 PATH 定位。
- 首次使用需安装 nmap：
  - Windows：`powershell -ExecutionPolicy Bypass -File scripts/setup-nmap.ps1`（下载官方便携版到 `tools/nmap/windows/`）
  - Linux/macOS：`bash scripts/setup-nmap.sh`（通过系统包管理器/Homebrew 安装）
  - `tools/nmap/` 目录已加入 `.gitignore`，便携二进制不入库
- 每次扫描结果与资产台账按 IP 比对，产出四类结果：**新增**（未匹配主机，默认人工确认后纳管，规则可开自动纳管）、**变更**（端口/主机名/OS 变化，默认人工确认后应用）、**恢复在线**、**离线**（连续两轮未发现才判定，防止误报）。
- **扫描算法（两阶段 + 分片并行）**：目标展开为具体 IP 列表（受 `max_hosts` 上限保护）；单目标规则直接详扫；大网段先用**探活端口**（规则 `probePorts`，默认 `22,80,443,445,3389`）快速定位存活主机，再仅对存活主机做全端口详扫；超过 `scan_chunk_size` 的主机数自动分片，`max_parallel_scans` 并发执行，全片失败才判定扫描失败。
- **匹配增强（MAC + 多 IP）**：同网段扫描可获取 ARP MAC，比对时按 **MAC → IP** 顺序匹配（MAC 归一化忽略大小写与分隔符）；资产支持**关联 IP**（`additionalIPs`，多网卡场景），任一关联 IP 命中即视为该资产在线，其余 IP 一并标记防止误判离线。
- 纳管与变更应用会写入资产快照与审计日志；资产详情页“变更历史”时间线展示字段级 diff。
- 资产新增在线状态（`onlineStatus`: online/offline/unknown）与最近发现时间（`lastSeenAt`），资产列表支持按在线状态筛选。
- 手动编辑资产、工单关闭写回资产也会生成快照，纳入变更历史。
- 调度器为进程内 ticker（按规则间隔分钟执行），进程重启后按 `lastRunAt` 自然恢复节奏；`POST /discovery/rules/:id/test` 可试跑并检查 nmap 可用性。
- **变化检测精细化（风险分级 + 明细）**：变更 diff 输出**端口新增/关闭明细**（如 `开放端口: 新增 8080; 关闭 22`）与**服务版本 diff**（按端口对齐）；变更**风险分级**——仅端口新增为**低风险**（规则开启「自动应用」后自动写入台账，记审计与快照），端口关闭 / 主机名 / OS / 服务版本变化为**高风险**（需人工在运行详情勾选确认）；离线同样视为高风险。
- **离线判定增强（时间窗口 + 跨规则确认）**：资产 `lastSeenAt` 距今超过 `offline_after_hours`（默认 24 小时）且**最近一次任意规则**的成功运行中该 IP 仍缺席才判定离线，避免单轮误报与“另一网段仍在线”误判。

相关配置：

```yaml
discovery:
  nmap_bin: ""            # 空=自动探测 tools/nmap/<platform>/ 与 PATH
  scan_timeout_sec: 300   # 单次扫描超时
  default_ports: "22,80,443,3389"
  probe_ports: "22,80,443,445,3389"
  max_parallel_scans: 4   # 大网段分片并行扫描最大并发
  scan_chunk_size: 128    # 大网段单次扫描主机数上限
  max_hosts: 1024         # 单次扫描最大主机数，防止误配大网段
  offline_after_hours: 24 # 资产连续未响应超过该小时数（且跨规则最近一轮缺席）才判离线
```

## v2.6 UI 现代化（明亮现代风 + 动效）

- 前端整体 UI 升级为**明亮现代风**：品牌渐变主色（indigo→purple）、大圆角、精致阴影、柔和渐变页面背景，通过 Element Plus CSS 变量主题覆盖实现，全站组件（按钮/表格/标签/弹窗/输入框/标签页）统一焕新。
- **布局**：侧边栏渐变 Logo 区、菜单项圆角悬浮 + 当前项渐变高亮；顶栏毛玻璃（backdrop blur）+ 渐变页标题 + 圆形用户头像。
- **登录页**：品牌光晕漂浮背景 + 毛玻璃卡片弹入动画 + 图标输入框 + 渐变主按钮。
- **全局动效**：路由切换淡入位移过渡、面板/卡片 hover 上浮、表格行 hover 微移、主按钮渐变悬浮阴影、输入框聚焦光晕等（纯 CSS，零新增依赖）。
- **资产详情弹窗**：由侧滑抽屉改为**居中弹出式对话框**（`AssetDetailDialog`）：Hero 渐变头卡（主机名/IP/资产编号 + 在线状态呼吸灯徽章）、指标卡片（开放端口数滚动展示）、端口渲染为按服务类型着色的胶囊（悬停显示服务版本）、服务识别覆盖率进度条、变更历史时间线彩色发光节点 + 卡片化条目 + 入场 stagger 动画。
