> **🤖 AI AGENTS — READ THIS FIRST**
> Before making any changes to this codebase, you MUST read [`agents.md`](./agents.md).
> It contains: tech stack baseline (do NOT upgrade), architecture rules, forbidden patterns, naming conventions, test requirements, and migration rules.

---

# XMECO 多智能体能效节能系统

## 项目概述

XMECO（熊猫智控）是一套面向中央空调水冷系统的 AIOT 能效管理平台，由以下模块组成：

| 模块 | 技术栈 | 端口 | 说明 |
|---|---|---|---|
| 后端 API | Go 1.25 + PostgreSQL | 9090 | RESTful API，JWT 认证，RBAC 权限 |
| Web 大屏 | React 19 + Ant Design + Vite | 3001 | 深色全屏大屏，设备拓扑图 + 实时控制 |
| Web 管理后台 | React 19 + Ant Design + Vite | 3000 | 管理员后台，15 个功能页面 |
| 小程序 | uni-app (Vue 3) | 5173 | 移动端运维，支持 H5/微信 |

---

## 一键启动

```bash
# 双击运行 — 自动启动数据库 + 后端 + 管理后台 + 大屏
d:\py\xmeco-new\start.bat
```

### 手动启动

```bash
# 1. 启动 PostgreSQL (Docker)
docker start xmeco-pg

# 2. 后端（迁移自动运行）
cd d:\py\xmeco-new
.\server.exe

# 3. Web 管理后台 (端口 3000)
cd web\admin && npm run dev

# 4. Web 大屏 (端口 3001)
cd web\admin && npm run dev:screen
```

### 访问地址

| 服务 | 地址 |
|------|------|
| Web 大屏 | http://localhost:3001 |
| Web 管理后台 | http://localhost:3000 |
| 小程序 H5 | http://localhost:5173 |

### 登录凭据

```
用户名: admin    密码: admin123    角色: 超级管理员 (super_admin)
```

---

## 目录结构

```
xmeco-new/
├── start.bat                            # 一键启动脚本
├── cmd/server/main.go                   # 后端入口 + 58 条路由注册
├── internal/
│   ├── api/handler/                     # 12 个 HTTP handler
│   │   ├── admin.go                     # 用户/代理商/角色/权限 CRUD + ResetPassword
│   │   ├── alarm.go                     # 告警规则 + 日志
│   │   ├── auth.go                      # 登录 + Me
│   │   ├── dashboard.go                 # 大屏 ScreenData 聚合端点 + 仪表盘配置
│   │   ├── intelligence.go              # 智能分析 + 电能质量 + 电表设备列表
│   │   ├── log.go                       # 遥测/控制记录/统计 + CSV 导出
│   │   ├── models.go                    # Building/Device/Property/Register CRUD
│   │   ├── project.go                   # 项目 CRUD + 用户分配
│   │   ├── startup.go                   # 启停计划 + 执行 + 定时任务 CRUD
│   │   └── telemetry.go                 # 实时/历史遥测
│   ├── api/middleware/
│   │   ├── auth.go                      # JWT AuthMiddleware + GetClaims
│   │   ├── cors.go                      # CORS 跨域中间件
│   │   ├── ratelimit.go                 # 登录速率限制
│   │   └── rbac.go                      # RequirePermission 权限中间件
│   ├── domain/                          # 领域模型
│   ├── repository/postgres/             # 数据访问层 (DBTX 接口 + pgxmock 测试)
│   ├── service/
│   │   ├── alarm/engine.go              # 告警引擎 (gt/ge/lt/le/eq/range) + 离线告警
│   │   ├── auth/                        # bcrypt 认证 + JWT + RBAC
│   │   ├── external/weather/            # 天气服务 (wttr.in + DB 缓存 60min)
│   │   ├── intelligence/                # 智能分析引擎
│   │   │   ├── analysis.go              # RunFullAnalysis + RunStrategies
│   │   │   ├── efficiency.go            # 设备能效分析
│   │   │   ├── engine.go                # 冷却塔-主机联动策略 + 湿球计算
│   │   │   ├── power_quality.go         # 电能质量分析 (GB/T 国标)
│   │   │   ├── predict.go               # 24h 负荷预测
│   │   │   ├── pump_price.go            # 泵变频优化 + 时段电价策略
│   │   │   ├── recommend.go             # 设定点推荐
│   │   │   └── rotation.go              # 设备轮换策略
│   │   ├── migration/                   # 自动迁移 (9 个版本，Go embed)
│   │   ├── orchestrator/                # 启停编排
│   │   └── telemetry/poller.go          # Modbus 遥测轮询 (含 decodeVal 管道)
│   ├── gateway/                         # 网关管理
│   │   ├── manager.go                   # Custom + DTU 双协议接入
│   │   ├── modbus/                      # Modbus RTU (CRC16/读写/解析)
│   │   └── transport/                   # Transport 接口 + Custom/Transparent 实现
│   └── config/config.go                 # 环境变量配置
├── web/admin/
│   ├── index.html                       # 管理后台入口 (端口 3000)
│   ├── vite.config.ts
│   ├── src/
│   │   ├── App.tsx                      # 后台路由 (/login → /projects → 14 页)
│   │   ├── pages/
│   │   │   ├── Login.tsx                # 后台登录页
│   │   │   ├── Screen.tsx               # 大屏组件 (深色全屏 + 登录 + 设备控制)
│   │   │   ├── Projects.tsx             # 项目管理 (含用户分配)
│   │   │   ├── Buildings.tsx            # 楼宇管理
│   │   │   ├── Devices.tsx              # 设备管理
│   │   │   ├── Properties.tsx           # 属性配置
│   │   │   ├── Registers.tsx            # 寄存器管理
│   │   │   ├── Alarms.tsx               # 告警管理
│   │   │   ├── Logs.tsx                 # 日志管理
│   │   │   ├── StartupPlans.tsx         # 启停配置 + 定时任务
│   │   │   ├── Users.tsx                # 用户管理
│   │   │   ├── Agents.tsx               # 代理商管理
│   │   │   ├── Permissions.tsx          # 权限管理
│   │   │   └── MultiAgent.tsx           # 多智能体 + 电能质量
│   │   ├── layouts/Main.tsx             # 侧边栏布局
│   │   ├── api/client.ts               # Axios 客户端
│   │   └── components/ProtectedRoute.tsx
│   └── screen-src/                      # 大屏独立入口 (端口 3001)
│       ├── index.html
│       ├── main.tsx
│       ├── ScreenApp.tsx
│       └── vite.config.ts
└── miniapp/                             # 小程序
```

---

## 数据库设计

### 核心表 (22 张)

| 表名 | 说明 |
|---|---|
| `agent` | 代理商 |
| `project` | 项目 |
| `project_user` | 项目-用户分配 (大屏/小程序权限控制) |
| `city` | 城市 (300+ 城市) |
| `building` | 楼宇 |
| `device` | 设备 |
| `device_properties` | 设备属性 (含关键属性 is_key) |
| `register` | 寄存器配置 (读/写地址+码+类型+字节序+掩码+倍率+状态码) |
| `users` | 用户 |
| `role` | 角色 (8 级) |
| `permission` | 权限点 (37 个) |
| `role_permission` | 角色-权限关联 |
| `alarm_rule` | 告警规则 (gt/ge/lt/le/eq/range) |
| `alarm_log` | 告警日志 |
| `control_record` | 控制记录 |
| `startup_plan` | 启停计划 |
| `startup_step` | 启停步骤 |
| `scheduled_task` | 定时任务 (once/daily/weekly) |
| `device_telemetry` | 设备遥测时序数据 |
| `dashboard_config` | 仪表盘配置 |
| `weather_cache` | 天气缓存 (60min TTL) |
| `schema_migrations` | 迁移版本追踪 |

### 迁移版本

| 版本 | 内容 |
|---|---|
| 001 | 初始 Schema (19 张表) |
| 002 | 种子数据 (角色+权限) |
| 003 | 仪表盘配置 |
| 004 | 天气缓存 |
| 005 | 城市数据 |
| 006 | gateway_imei VARCHAR(20→64) |
| 007 | device.last_online_at |
| 008 | scheduled_task 定时任务表 |
| 009 | project_user 用户分配表 |

---

## API 端点 (58 条路由)

### 大屏专用
```
GET /api/v1/screen/data           # 大屏聚合数据 (全部所需数据一次返回)
```

### 认证
```
POST /api/v1/auth/login           # 登录
GET  /api/v1/auth/me               # 当前用户信息
```

### 业务 CRUD
```
GET|POST          /api/v1/projects
GET|PUT|DELETE    /api/v1/projects/{id}
GET|PUT           /api/v1/projects/{id}/users    # 项目用户分配
GET|POST          /api/v1/buildings
GET|PUT|DELETE    /api/v1/buildings/{id}
GET|POST          /api/v1/devices
GET|PUT|DELETE    /api/v1/devices/{id}
POST              /api/v1/devices/{id}/control
GET|POST          /api/v1/properties
GET|PUT|DELETE    /api/v1/properties/{id}
GET|POST          /api/v1/registers
GET|PUT|DELETE    /api/v1/registers/{id}
GET|POST          /api/v1/alarm-rules
PUT|DELETE        /api/v1/alarm-rules/{id}
GET               /api/v1/alarm-logs
POST              /api/v1/alarm-logs/{id}/ack
GET|POST          /api/v1/startup-plans
PUT|DELETE        /api/v1/startup-plans/{id}
POST              /api/v1/startup-plans/{id}/execute
GET               /api/v1/startup-executions/{id}
POST              /api/v1/startup-executions/{id}/stop
```

### 定时任务
```
GET|POST          /api/v1/scheduled-tasks
PUT|DELETE        /api/v1/scheduled-tasks/{id}
```

### 遥测 & 日志
```
GET /api/v1/telemetry/realtime
GET /api/v1/telemetry/history
GET /api/v1/telemetry/stats
GET /api/v1/logs/telemetry         # 支持 interval=raw/minute/hour/day/week/month/year
GET /api/v1/logs/controls
GET /api/v1/logs/stats
```

### 天气
```
GET /api/v1/weather/cities
GET /api/v1/weather/cities/{id}
GET /api/v1/weather/provinces
GET /api/v1/weather/now
GET /api/v1/weather/project
```

### 智能分析
```
GET /api/v1/intelligence/full
GET /api/v1/intelligence/efficiency
GET /api/v1/intelligence/forecast
GET /api/v1/intelligence/recommendations
GET /api/v1/intelligence/strategies
GET /api/v1/intelligence/price-config
PUT /api/v1/intelligence/price-config
GET /api/v1/intelligence/power-quality     # 电能质量分析 (电压/电流/THD/功率因数/频率)
GET /api/v1/intelligence/meter-devices     # 电表设备列表
```

### 权限管理
```
GET|POST          /api/v1/users
PUT               /api/v1/users/{id}
POST              /api/v1/users/{id}/reset-password
DELETE            /api/v1/users/{id}
GET|POST          /api/v1/agents
PUT|DELETE        /api/v1/agents/{id}
GET               /api/v1/roles
GET               /api/v1/permissions
GET               /api/v1/roles/{id}/permissions
PUT               /api/v1/roles/{id}/permissions
```

### 导出 & 系统
```
GET /api/v1/export/telemetry
GET /api/v1/export/controls
GET /api/v1/system/info
GET /api/v1/system/db-stats
GET /api/v1/dashboard
PUT /api/v1/dashboard
GET /api/v1/gateways
GET /api/health
```

---

## Web 大屏功能

大屏为深色全屏独立应用 (端口 3001)，内置登录，提供：

| 区域 | 功能 |
|---|---|
| 顶部栏 | 项目/楼宇下拉选择 + 代理商名 + 用户名 + 退出 |
| 导航 Tab | 监控中心/数据中心/维保中心/任务中心/决策中心/系统日志/能耗中心 |
| 左侧面板 | 今日天气 + 定时任务 + 故障报警 |
| 中央拓扑 | 冷却塔→冷却泵→主机→阀门→冷冻泵→二次泵，设备数量自适应排列 |
| 设备交互 | 点击设备查看属性；开关机可 Switch 控制；数值可 InputNumber 调节 (min~max) |
| 关键属性 | 属性设为"关键"后直接显示在设备方块下方 |
| 右侧面板 | 节能率/节电量/节碳量/运行时长/电能统计 |

---

## Web 后台路由

```
/login          → 登录页
/projects       → 项目管理 (城市级联 + 用户分配)
/buildings      → 楼宇管理
/devices        → 设备管理
/properties     → 属性配置 (含关键属性标识)
/registers      → 寄存器管理
/alarms         → 告警管理
/startup-plans  → 启停配置 + 定时任务
/logs           → 日志管理 (分钟/小时/天/周/月/年)
/users          → 用户管理
/agents         → 代理商管理
/permissions    → 权限管理
/multi-agent    → 智能分析 + 电能质量
```

---

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `XMECO_DB_HOST` | localhost | 数据库主机 |
| `XMECO_DB_PORT` | 5432 | 数据库端口 |
| `XMECO_DB_USER` | postgres | 数据库用户 |
| `XMECO_DB_PASSWORD` | xmeco123 | 数据库密码 |
| `XMECO_DB_NAME` | xmeco | 数据库名 |
| `XMECO_SERVER_PORT` | 9090 | HTTP 端口 |
| `XMECO_JWT_SECRET` | xmeco-dev-secret... | JWT 签名密钥 |
| `XMECO_RETENTION_DAYS` | 730 | 遥测数据保留天数 (0=不清理) |
| `XMECO_POLL_INTERVAL_SEC` | 3 | 设备轮询间隔秒数 |

---

## 架构亮点

### 1. 自动迁移
- Go embed 嵌入 9 个 SQL 迁移文件
- `schema_migrations` 表追踪版本
- `detectExistingVersion` 智能检测已有表 → 自动跳过已应用版本
- 全部幂等 (IF NOT EXISTS/ON CONFLICT DO NOTHING)

### 2. 大屏-后台隔离
- 前台 `/` 直连大屏 (端口 3001)
- 后台 `/admin/*` 走管理后台 (端口 3000)
- 各自独立的 Vite 配置和 HTML 入口
- 大屏 ScreenData 聚合端点一次返回全部数据

### 3. 多智能体引擎
- 设备能效排行 + 24h 负荷预测 + 设定点推荐
- 湿球温度 (Stull 公式) + 分时电价策略
- 电能质量分析 (电压/电流/THD/功率因数/频率 — GB/T 14549/12325/15945)
- 冷却塔-主机联动 + 泵阀频率优化 + 设备轮换

### 4. 安全设计
- bcrypt 密码哈希 (cost=10)
- JWT 认证 (HS256, 含过期校验)
- RBAC 37 权限码 × 8 角色
- 登录频率限制 (10次/分钟/IP)
- 项目用户分配 (大屏/小程序访问控制)

### 5. 代码质量
- NULL-COALESCE 全覆盖 (100+ 列安全扫描)
- DBTX 接口解耦 pgxpool (42 个 mock 测试)
- 表驱动测试 + 边界覆盖
- slog 日志 (Warn 级别 scan 失败 + 离线告警)
- Goroutine panic recovery (retention/offline/scheduler 三路守护)
- 批量 SQL (markOnline `WHERE id = ANY($1)`)
- 定时任务精确匹配 (weekly 用 `regexp_split_to_array` 防子串误匹配)

### 6. 运维守护
- 数据保留：启动立即执行清洁，此后每 24h 分片删除过期遥测 (10000行/批)
- 离线检测：启动立即扫描，此后每 1min 检测 `last_online_at < NOW()-10min` → 告警 + 离线
- 定时任务：每 1min 执行 due tasks (once/daily/weekly)，once 类型 `last_run_at IS NULL` 确保只跑一次

---

## 工程指标

| 维度 | 数据 |
|------|------|
| Go 源文件 | 55+ |
| 测试文件 | 11 |
| 测试用例 | ~170 (0 失败) |
| 测试覆盖包 | 12/15 |
| Web 页面 | 15 (大屏 + 14 后台页) |
| API 路由 | 58 条 |
| 数据库表 | 23 张 |
| 迁移版本 | 9 |
| `go vet` | clean ✅ |
| `go build` | OK ✅ |
| Lint | 零 WARNING ✅ |