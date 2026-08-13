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
  min_rate: 0             # nmap --min-rate（每秒发包下限），0=不限制
  max_rate: 0             # nmap --max-rate（每秒发包上限），0=不限制
  alert_emails: ""        # 变更告警收件人（逗号分隔），空=不发送
  offline_after_hours: 24 # 资产连续未响应超过该小时数（且跨规则最近一轮缺席）才判离线
```

### 资产智能（P5-P8）

- **指纹推断资产类型**：纳管/应用时按端口、服务、OS 自动推断 `assetType`（server/database/network/workstation），仅在类型为空时回填，不覆盖人工填写；资产列表与详情以彩色标签展示。
- **异常端口预警**：应用变更时对比资产历史快照端口基线，新开放高危端口（22/3389/445/3306/5432/6379/27017/9200/11211/8080/9090）写入 `alert` 审计日志。
- **扫描治理**：规则可限定**扫描时段**（`scanWindowStart/End`，支持跨天）、**增量扫描**（仅重扫上次存活主机）；全局 `min_rate`/`max_rate` 控制 nmap 发包速率。
- **变更告警通知**：配置 `alert_emails` 后，运行产生新增/变更/离线时发送告警邮件（复用系统邮件配置，失败仅记日志）。
- **高风险变更自动生成工单**：规则开启「自动生成工单」后，高风险变更自动创建资产变更工单（草稿）进入审批流，工单关闭时写回资产台账。
- **可视化与报表**：数据看板首页（资产概览/类型分布/发现趋势/最近运行）、发现趋势统计（`/discovery/stats/trend`）、网段分布（`/discovery/stats/subnets`）、端口/服务矩阵热力图（`/discovery/stats/services`）、资产统计报表导出（`/assets/stats/export`）、操作审计日志页（`/audit-logs`）。
- **资产生命周期**：资产详情支持**退役归档**（状态置 retired、保留快照与审计）、展示**使用年限**（按 `purchaseDate` 计算）。

## v2.16 机柜视图

- **机柜 U 位可视化**：「机柜视图」页面（ECharts）选择机房 → 机柜卡片 → 机柜正视图，按 U 位数渲染网格，资产按 `rackPosition`（`12` 单 U / `12-14` 多 U）色块定位，tooltip 显示主机名/IP/类型，点击跳转资产详情。
- 资产筛选接口新增 `rack`/`location` 参数；资产编辑表单补机房/机柜/U 位下拉（机柜按机房联动），详情展示机柜位置。

## v2.15 机房机柜管理

- **机房/机柜实体**：新增机房（`datacenter_rooms`）与机柜（`racks`，U 位数默认 42）模型，`/rooms`、`/racks` CRUD（admin/资产管理员），删除机房连带删除其机柜。
- **管理页**：「机房机柜」页面左右双栏管理机房与机柜。
- 资产仍沿用 `location`（机房）/`rack`（机柜）/`rackPosition`（U 位）文本字段，按名称与机柜实体匹配挂靠。

## v2.14 资产盘点

- **盘点单**：「资产盘点」页面（admin/资产管理员）创建盘点单，可按资产类型过滤范围，创建时对资产快照生成盘点明细。
- **逐项核对**：盘点明细逐项标记「一致 / 盘亏」并填写原因；关闭盘点单后统计总数/已核对/一致/盘亏，并导出 CSV 差异报告。
- 相关接口：`/stocktakes` CRUD、`/stocktakes/:id/close`、`/stocktakes/:id/export`。

## v2.13 维保到期提醒

- **自动提醒**：进程内维保扫描器每天检查一次，对维保到期日在未来 30 天内（含已过期）的资产发送**邮件**（收件人为系统管理员）与 **IM 群通知**，列出主机名/IP/到期日/维保厂商。
- **前端展示**：资产列表新增「维保到期」列，已过期红色、30 天内「即将到期」黄色高亮。

## v2.12 IM 回调验签（自建应用审批）

- **回调验签配置**：系统配置 → IM 通知新增「回调验签配置」区块，按平台配置验签密钥（钉钉 AppSecret / 企微 Token+EncodingAESKey+CorpID / 飞书应用 secret），敏感字段 AES-GCM 加密存储。
- **平台原生验签**：`POST /api/v1/im/callback` 按 URL/header 特征分派——钉钉 `signature/timestamp`（HMAC-SHA256）、企微 `msg_signature` + WXBizMsgCrypt AES 解密、飞书 `X-Lark-Signature`；验签通过后进入 IM 用户绑定鉴权与工单状态机流转。
- 兜底保留通用共享密钥 HMAC 验签（`X-IM-Sign`/`X-IM-Timestamp`）。

## v2.11 工单统计报表

- **统计报表页**：「工单报表」页面（admin）展示：工单总数、SLA 达标率（已关闭且带 SLA 截止时间的工单，达标 = 归档时间不晚于截止）、SLA 超时数；状态/类型/优先级分布（CSS 条形图）、近 12 个月创建趋势（柱状图）。
- **CSV 导出**：`GET /tickets/stats/export` 导出汇总（状态/类型/优先级/SLA 达标率/月度趋势）。
- 相关接口：`GET /tickets/stats`（admin）。

## v2.10 定期巡检 / 循环工单

- **巡检规则**：「巡检规则」页面（admin）维护规则：名称、巡检内容说明、频率（每天/每周/每月）、执行时刻、周执行日或月执行日、巡检执行人、启停。
- **自动建单**：进程内巡检调度器每分钟检查启用的规则，到达本周期执行时刻且未生成过时自动创建**巡检工单（草稿）**——申请人取首位 admin，执行人取规则指定人，工单描述写入巡检内容；生成后记录 `lastRunAt`，同一周期不重复生成。
- **试运行**：`POST /inspection/rules/:id/test` 立即生成一张巡检工单验证配置。
- 巡检工单类型 `inspection`（字典「定期巡检」），走现有工单状态机：提交审批 → 审批 → 执行 → 待验收 → 关闭归档。
- 相关接口：`/inspection/rules` CRUD（admin）。

## v2.9 工单多资产关联

- **多资产选择**：新建/编辑工单时可按 `assetIds` 关联多个受影响资产（兼容旧的单资产 `assetId` 字段，为空时自动回退），关联关系存于 `ticket_assets` 表。
- **详情展示**：工单详情展示关联资产清单（主机名/资产编号 + IP）。
- **验收写回**：工单验收关闭时对**全部关联资产**写回变更字段（设备类型/主机名/IP/端口/服务/版本/厂商），逐台生成资产快照与审计；无关联资产时回退单资产逻辑。
- **归档 PDF**：归档表增加「关联资产清单」行（IP / 名称 / 资产类型）。

## v2.8 工单 SLA 时效管理

- **流程级 SLA 配置**：「流程配置」页按工单类型配置**审批时限**与**完成时限**（小时，留空=不启用），存于流程配置（`TicketWorkflow`）。
- **自动计时**：工单**提交审批**时按该类型流程的审批时限写入审批截止时间（重新提交会重新计时）；**开始执行**时写入完成截止时间与执行开始时间。
- **超时扫描与通知**：进程内 SLA 扫描器每分钟检查处于「审批中/执行中」且已过截止时间的工单，标记超时并发送**邮件**（申请人 + 当前审批人/执行人）与 **IM 群通知**，写审计记录（`sla_overdue`）；每个工单仅通知一次，防止重复打扰。
- **前端展示**：工单列表新增「SLA」列，实时显示剩余/已超时时间（剩余绿色、超时红色高亮）；工单详情展示 SLA 时效。
- 对应模型字段：工单快照 `slaApprovalDeadline` / `slaCompletionDeadline` / `slaStartedAt` / `slaOverdueNotified`。

## v2.7 IM 群机器人通知（钉钉 / 企业微信 / 飞书）

- **群机器人通知**：系统配置页新增「IM 通知」tab，配置平台 + webhook（+ 钉钉加签密钥）即可启用；工单**创建 / 提交待审批 / 审批通过 / 驳回 / 验收关闭**事件自动推送群卡片，支持「查看详情」跳转按钮。
- **测试发送**：`POST /settings/im/test` 发送测试卡片验证配置。
- **用户绑定与回调网关**：`/settings/im/bindings` 维护 IM 用户 ↔ 系统用户映射；`POST /im/callback` 提供统一回调入口（飞书 challenge 验证；`approve`/`reject` 交互动作经绑定鉴权后调用现有工单状态机流转）。真正的 IM 内按钮审批需平台**自建应用** + 公网回调地址，接入时补充平台验签即可复用本网关。

### 接入指南

#### 档位 1：群机器人通知（内网可用，配置即用）

**钉钉**
1. 钉钉建群 → 群设置 → 智能群助手 → 添加机器人 → **自定义**；
2. 安全设置勾选**加签**，复制 `SEC` 开头密钥（建议必配）；
3. 复制 Webhook：`https://oapi.dingtalk.com/robot/send?access_token=xxx`；
4. 系统配置 → IM 通知：平台=钉钉，填 Webhook 与加签密钥，启用 → **发送测试**，群内收到卡片即成功。

**企业微信**
1. 企微内部群 → 群设置 → 群机器人 → 新建，复制 Webhook：`https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx`；
2. 系统配置 → IM 通知：平台=企业微信，填 Webhook，启用 → 测试（密钥留空即可）。

#### 档位 2：IM 内按钮审批（需平台自建应用 + 公网 HTTPS 回调）

回调统一入口：`POST /api/v1/im/callback`（当前实现：飞书 challenge 应答；`approve`/`reject` 交互动作要求 `X-IM-Sign = hex(hmac-sha256(secret, timestamp||body))` + `X-IM-Timestamp`（300s 窗口），经 IM 用户绑定鉴权后调用工单状态机流转）。

**钉钉自建应用**
1. [钉钉开放平台](https://open.dingtalk.com) 创建**企业内部应用**，记录 `AppKey`/`AppSecret`；
2. 事件订阅配置回调 URL 为上述地址并完成地址验证（网关已应答 challenge）；
3. 应用内发送带按钮的 actionCard，`actionURL` 指向回调地址；
4. 系统配置 → IM 通知 → 用户绑定：钉钉 openId ↔ 系统用户；
5. 接入时在 `IMCallback` 前补钉钉事件验签（`timestamp`+`sign` 头）分支。

**企业微信自建应用**
1. [企微管理后台](https://work.weixin.qq.com) 应用管理 → 创建自建应用，获得 `AgentId`/`Secret`/`CorpId`；
2. 应用 → 接收消息 → 设置 API 接收：URL 同上，Token/EncodingAESKey 按企微生成；
3. **企微回调采用强制加解密协议**（WXBizMsgCrypt），接入时在网关实现解密后再进入动作处理；
4. 系统配置 → 用户绑定：企微 userId ↔ 系统用户。

**飞书**：开放平台创建企业自建应用，事件订阅回调 URL 同上，验签头 `X-Lark-Signature` + timestamp。

#### 两种方式速查

| | 群机器人（档位 1，已就绪） | 自建应用审批（档位 2，待平台验签） |
|---|---|---|
| 依赖 | 无（内网可用） | 公网 HTTPS 回调地址 |
| 能力 | 群内通知 + 查看详情跳转 | IM 内点按钮直接审批 |
| 配置 | 系统配置页 5 步 | 平台开放平台 + 系统用户绑定 |
| 验签 | 已实现（HMAC 共享密钥） | 需按平台协议补充（钉钉 sign / 企微 WXBizMsgCrypt / 飞书 X-Lark-Signature） |

## v2.6 UI 现代化（明亮现代风 + 动效）- 前端整体 UI 升级为**明亮现代风**：品牌渐变主色（indigo→purple）、大圆角、精致阴影、柔和渐变页面背景，通过 Element Plus CSS 变量主题覆盖实现，全站组件（按钮/表格/标签/弹窗/输入框/标签页）统一焕新。
- **布局**：侧边栏渐变 Logo 区、菜单项圆角悬浮 + 当前项渐变高亮；顶栏毛玻璃（backdrop blur）+ 渐变页标题 + 圆形用户头像。
- **登录页**：品牌光晕漂浮背景 + 毛玻璃卡片弹入动画 + 图标输入框 + 渐变主按钮。
- **全局动效**：路由切换淡入位移过渡、面板/卡片 hover 上浮、表格行 hover 微移、主按钮渐变悬浮阴影、输入框聚焦光晕等（纯 CSS，零新增依赖）。
- **资产详情弹窗**：由侧滑抽屉改为**居中弹出式对话框**（`AssetDetailDialog`）：Hero 渐变头卡（主机名/IP/资产编号 + 在线状态呼吸灯徽章）、指标卡片（开放端口数滚动展示）、端口渲染为按服务类型着色的胶囊（悬停显示服务版本）、服务识别覆盖率进度条、变更历史时间线彩色发光节点 + 卡片化条目 + 入场 stagger 动画。
