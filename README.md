<p align="center">
  <img src="web/public/logo.svg" width="92" height="92" alt="HopTrace Logo" />
</p>

<h1 align="center">HopTrace</h1>

<p align="center">
  <strong>轻量级企业网络拓扑自动发现、终端定位与运维管控平台</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/Kratos-v3-blue" alt="Kratos v3" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?logo=react" alt="React 19" />
  <img src="https://img.shields.io/badge/Ant%20Design%20Pro-6-1677FF?logo=antdesign" alt="Ant Design Pro 6" />
  <img src="https://img.shields.io/badge/AntV%20G6-v5-722ED1" alt="AntV G6 v5" />
</p>

---

## 项目简介

**HopTrace** 面向企业园区网与多机房/办公区场景，基于 **SNMP v2c/v3 + LLDP + FDB/ARP + SNMP Trap/Syslog + NETCONF/SSH**，实现从核心/防火墙到接入交换机与终端设备的全链路自动发现、可视化拓扑绘制、终端位置追踪与安全端口管控。

针对国内主流 **华为（VRP 5/8，S/CE/USG/AR/AC 系列）** 设备做了深度 OID 校准与兼容（包括华为 `Eth-Trunk` 聚合、`hwDynFdbPort` VLAN 解析、`dot1qPvid` 兜底、无线 AC STA 采集等），同时兼容标准 RFC/IEEE MIB 设备。

> **内置华为风格模拟网络（`simulate: true`）**：克隆代码后无需真实交换机，即可一键体验完整的自动发现、拓扑绘制、终端定位、路径追踪与端口管控流程。

---

## 界面预览

### 总览仪表盘
![总览仪表盘](docs/screenshots/dashboard.png)

### 交互式网络拓扑（支持展开全部终端、防遮挡重绘、路径追踪）
![网络拓扑](docs/screenshots/topology.png)

### 网络设备管理（多机房/办公区归属、堆叠与健康状态）
![网络设备管理](docs/screenshots/devices.png)

### 接入终端台账与定位（OUI 厂商识别、VLAN/端口精确定位）
![接入终端管理](docs/screenshots/endpoints.png)

### IPAM 子网占用率与网络异常检测
![IPAM 与异常检测](docs/screenshots/ipam.png)

---

## 核心特性

- **多机房 / 办公区管理**：按机房、办公区、分支机构、园区维度划分并统计设备与终端，支持批量划入/移出及专属拓扑视图。
- **自动发现与设备指纹**：支持种子 IP + LLDP 递归扩散及网段 CIDR 扫描，自动识别设备型号、角色（防火墙/核心/汇聚/接入/路由/AC）、堆叠成员数与健康指标（CPU/内存/温度）。
- **交互式全屏拓扑（AntV G6 v5）**：
  - 支持 **层次 / 力导向 / 同心圆 / 自定义已保存布局** 自由切换，内置矩形防遮挡算法，展开终端不重叠。
  - 支持 LLDP + FDB（AFT）拓扑融合与 `Eth-Trunk` 聚合链路合并。
  - 支持 **端到端路径追踪（Path Trace）**、角色过滤、按机房切换、PNG 导出。
  - 支持 **实时端口速率与带宽利用率** 悬停展示（包含接入交换机到终端的实时速率）。
- **终端全生命周期追踪**：自动关联 MAC/IP/VLAN/接入端口，内置 40,000+ IEEE OUI 厂商库，记录终端上线/下线/位置漂移/IP 变更历史，支持华为 AC 无线终端（SSID/AP/RSSI）采集与批量 CSV 导入导出。
- **实时动态感知（Trap + Syslog + SSE）**：内置 SNMP v2c/v3 Trap 监听器（`linkUp`/`linkDown` 触发去抖增量重扫）与 UDP Syslog 接收器，通过 **SSE** 实时推送刷新前端拓扑与仪表盘。
- **IPAM 与网络异常检测**：自动统计子网地址利用率，主动识别 **IP 冲突、疑似私接 HUB/小交换机、MAC 震荡漂移**。
- **告警中心与配置备份**：内置告警中心（支持企业微信/钉钉/飞书/通用 Webhook 推送）以及设备运行配置定时/手动备份、SHA-256 变更检测与行级 Diff 对比。
- **端口管控与审批流（NETCONF / SSH）**：支持端口开关、VLAN 切换、终端隔离，内置核心/上联/聚合口防误操作保护、Dry-Run 预览、审批流与一键回滚。
- **权限与安全**：SNMP/SSH 凭据采用 **AES-GCM** 加密落库，内置可选 JWT 鉴权与 `admin / operator / viewer` 三级 RBAC 权限模型。

---

## 技术栈与架构

| 层次 | 技术选型 |
| :--- | :--- |
| **后端框架** | Go + [Kratos v3](https://github.com/go-kratos/kratos) + Wire 依赖注入 + Protobuf / gRPC / HTTP |
| **数据存储** | GORM + SQLite（默认开箱即用）/ PostgreSQL（生产推荐，含月度分区脚本） |
| **网络采集** | `gosnmp` (SNMP v2c/v3 & Trap) + NETCONF over SSH / SSH CLI + UDP Syslog |
| **前端框架** | React 19 + TypeScript + Ant Design Pro 6 (`@umijs/max`) + Ant Design 6 |
| **拓扑引擎** | AntV G6 v5 (`dagre` / `d3-force` / `concentric` + 自定义防遮挡) |

### 目录结构

```text
hoptrace/
├── api/hoptrace/v1/        # Protobuf API 契约与生成的 HTTP/gRPC Stub
├── cmd/
│   ├── hoptrace/           # 主服务入口与 Wire 注入
│   └── hoptrace-walk/      # 离线 snmpwalk 回放校验工具
├── configs/                # 运行时配置 (config.yaml)
├── deploy/                 # PostgreSQL 分区等部署辅助脚本
├── internal/
│   ├── biz/                # 领域模型(DO)、业务逻辑、拓扑/终端引擎、仓储接口
│   ├── conf/               # 配置 Protobuf 定义
│   ├── data/               # GORM 持久化(PO)、SNMP 采集器、内置华为模拟器
│   ├── pkg/oui/            # 内置 IEEE OUI 厂商识别库
│   ├── server/             # HTTP/gRPC/SSE/Trap/Syslog 服务与鉴权中间件
│   └── service/            # Transport 适配层 (DTO ↔ DO)
└── web/                    # Ant Design Pro 6 前端工程
```

---

## 环境要求

- **Go**：1.26 及以上（见 [go.mod](go.mod)）
- **Node.js**：20+ 及 **pnpm**（推荐）或 npm
- **可选（仅修改 `.proto` 或 Wire 注入时需要）**：`protoc`、`buf`、`wire`

---

## 快速开始（本地开发运行）

### 1. 克隆仓库

```bash
git clone https://github.com/go-laoji/hoptrace.git
cd hoptrace
```

### 2. 启动后端服务

默认配置 [configs/config.yaml](configs/config.yaml) 使用 SQLite（自动建表）并开启了模拟网络模式（`collector.simulate: true`），直接运行即可：

```bash
go run ./cmd/hoptrace -conf ./configs/config.yaml
```

启动后默认监听：
- HTTP API & Web 托管：`http://localhost:8000`
- gRPC：`localhost:9000`
- SNMP Trap (UDP)：`0.0.0.0:1162`
- Syslog (UDP)：`0.0.0.0:1514`

### 3. 启动前端开发服务

另开一个终端进入 `web/` 目录：

```bash
cd web
pnpm install        # 或 npm install
pnpm run dev        # 或 npm run dev
```

浏览器访问前端开发地址（通常为 `http://localhost:8000` 或控制台输出的端口，`/api/*` 已自动代理至后端）。

> **快速体验模拟网络**：进入「**发现与凭据**」页面，点击右上角「**一键填入模拟网络种子 (10.255.0.1)**」并启动发现任务，数秒内即可在「**网络拓扑**」与「**接入终端**」中看到完整的模拟园区网。

---

## 生产环境编译与部署

HopTrace 支持将前端静态产物直接由 Go 单二进制托管（通过 `server.static_dir: web/dist`），部署非常简洁。

### 1. 编译前端

```bash
cd web
pnpm install --frozen-lockfile
pnpm run build
cd ..
```

构建产物位于 `web/dist/`。

### 2. 编译后端二进制

**本机平台编译：**

```bash
make build
# 产物位于 bin/hoptrace
```

**跨平台编译 Linux amd64 二进制（纯 Go，无需 CGO）：**

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/hoptrace-linux ./cmd/hoptrace
```

### 3. 准备配置并运行

复制并按需修改 [configs/config.yaml](configs/config.yaml)：

1. **接入真实网络**：将 `collector.simulate` 改为 `false`。
2. **开启登录鉴权（推荐生产开启）**：将 `security.auth_enabled` 改为 `true`，并通过环境变量设置加密密钥与初始管理员密码：
   ```bash
   export SECURITY_SECRET_KEY="your-strong-random-secret"
   export INIT_ADMIN_USER="admin"
   export INIT_ADMIN_PASSWORD="your-strong-password"
   ```
3. **切换 PostgreSQL（可选，大规模现网推荐）**：
   ```bash
   export DATABASE_DRIVER="postgres"
   export DATABASE_SOURCE="postgres://user:pass@127.0.0.1:5432/hoptrace?sslmode=disable"
   ```
4. **启动服务**：
   确保部署目录包含二进制文件、`configs/config.yaml` 以及前端 `web/dist/` 目录：
   ```bash
   ./bin/hoptrace -conf ./configs/config.yaml
   ```

浏览器直接访问 `http://<服务器IP>:8000` 即可使用。

---

## 现网设备配置提示（华为 VRP 示例）

1. **SNMPv3 视图范围**：若使用 SNMPv3，请确保 MIB 视图包含 `iso` 根节点（而不仅是 `1.3.6.1`），否则无法读取 LLDP（`1.0.8802`）与聚合口（`1.2.840`）信息：
   ```text
   snmp-agent mib-view included hoptrace-view iso
   ```
2. **SNMP Trap 推送（端口变化实时刷新拓扑）**：
   ```text
   snmp-agent trap enable
   snmp-agent target-host trap address udp-domain <HopTrace服务器IP> udp-port 1162 params securityname <凭据名> v2c
   ```
3. **离线 Walk 回放调试**：遇到新机型解析问题时，可使用内置工具回放 `snmpwalk` 文件验证解析结果：
   ```bash
   go run ./cmd/hoptrace-walk -file your-switch.walk
   ```

---

## 代码生成与测试

修改 `.proto` 或 Wire 依赖后执行：

```bash
make api        # 重新生成 api/hoptrace/v1/*.pb.go
make config     # 重新生成 internal/conf/*.pb.go
make generate   # 重新生成 wire_gen.go 并整理依赖
```

运行全量测试：

```bash
go test ./...
```

---

## 开源协议

欢迎提交 Issue 与 Pull Request 共同完善多厂商设备适配与运维能力。
