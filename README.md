# XMECO 多智能体能效节能系统

## 项目概述

XMECO（熊猫智控）是一套面向中央空调水冷系统的 AIOT 能效管理平台，由以下模块组成：

| 模块 | 技术栈 | 端口 | 说明 |
|---|---|---|---|
| 后端 API | Go 1.25 + PostgreSQL (TimescaleDB) | 9090 | RESTful API，JWT 认证，RBAC 权限 |
| Web 管理后台 | React 19 + Ant Design 6 + Vite 8 | 3000 | 管理员后台，15 个功能页面 |
| 小程序 | uni-app (Vue 3) + Vite | 5173 | 移动端运维，支持 H5/微信 |

---

## 快速启动

```bash
# 1. 启动 PostgreSQL (Docker)
docker start xmeco-pg

# 2. 后端（迁移自动运行，无需手动执行 SQL）
cd D:\py\xmeco-new
go build -o server.exe ./cmd/server/
./server.exe

# 3. Web 管理后台
cd web/admin && npm run dev

# 4. 小程序 (H5 开发)
cd miniapp && npm run dev:h5
```

### 访问地址

| 服务 | 地址 |
|------|------|
| API 健康检查 | http://localhost:9090/api/health |
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
├── cmd/server/main.go                  # 后端入口 + 路由注册
├── internal/
│   ├── api/
│   │   ├── handler/                    # 11 个 HTTP handler
│   │   │   ├── admin.go                # 用户/代理商/角色/权限 CRUD
│   │   │   ├── alarm.go                # 告警规则 + 日志
│   │   │   ├── auth.go                 # 登录 + Me
│   │   │   ├── dashboard.go            # 仪表盘配置
│   │   │   ├── intelligence.go         # 智能分析 + 协同控制
│   │   │   ├── log.go                  # 遥测/控制记录/统计
│   │   │   ├── models.go               # 公共辅助函数 + Building/Device/Property/Register CRUD
│   │   │   ├── project.go              # 项目 CRUD
│   │   │   ├── startup.go              # 启停计划 + 执行
│   │   │   ├── telemetry.go            # 实时/历史遥测
│   │   │   └── weather.go              # 城市 + 天气查询
│   │   └── middleware/
│   │       ├── auth.go                 # JWT AuthMiddleware + GetClaims
│   │       ├── cors.go                 # CORS 跨域中间件
│   │       ├── ratelimit.go            # 登录速率限制 (10次/分钟/IP)
│   │       └── rbac.go                 # RequirePermission 权限中间件
│   ├── domain/                         # 领域模型 (Project/Building/Device/City/WeatherNow)
│   ├── repository/postgres/            # 数据访问层 (db.go/models.go/project.go/admin.go)
│   ├── service/                        # 业务逻辑层
│   │   ├── alarm/engine.go             # 告警引擎 (gt/ge/lt/le/eq)
│   │   ├── auth/                       # 认证 (auth.go) + RBAC (rbac.go)
│   │   ├── external/weather/           # 天气服务 (wttr.in + DB 缓存)
│   │   ├── intelligence/               # 智能分析引擎
│   │   │   ├── analysis.go             # 入口：RunFullAnalysis + RunStrategies
│   │   │   ├── efficiency.go           # 设备能效分析
│   │   │   ├── engine.go               # 冷却塔-主机联动策略 + 湿球计算
│   │   │   ├── predict.go              # 24h 负荷预测
│   │   │   ├── pump_price.go           # 泵变频优化 + 时段电价策略
│   │   │   ├── recommend.go            # 设定点推荐
│   │   │   └── rotation.go             # 设备轮换策略
│   │   ├── migration/migrate.go        # 自动迁移 (Go embed + 版本追踪)
│   │   ├── orchestrator/               # 启停编排 (orchestrator.go + interlock.go)
│   │   └── telemetry/poller.go         # Modbus 遥测轮询
│   ├── gateway/                        # 网关管理
│   │   ├── manager.go                  # 网关连接管理 (Custom + DTU)
│   │   ├── modbus/                     # Modbus RTU 协议 (CRC16/读写命令/解析)
│   │   └── transport/                  # 传输层 (CustomTransport + TransparentTransport)
│   └── config/config.go                # 环境变量配置
├── web/admin/                          # Web 管理后台 (端口 3000)
│   └── src/
│       ├── pages/                      # 14 个页面 (见下方路由表)
│       ├── layouts/Main.tsx            # 侧边栏 + 顶栏布局
│       ├── api/client.ts               # Axios API 客户端
│       └── components/ProtectedRoute.tsx
└── miniapp/                            # 小程序 (端口 5173)
    └── src/
        ├── pages/                      # 6 个页面 (index/devices/alarms/mine/detail/login)
        ├── api/client.ts               # uni.request API 客户端
        └── static/                     # tabBar 图标
```

---

## 数据库设计

### 核心表 (19 张)

| 表名 | 说明 | 行数 |
|---|---|---|
| `agent` | 代理商 | 2 |
| `project` | 项目 (关联 agent, city) | 1 |
| `city` | 城市 (300+ 城市，含行政区划代码) | 300+ |
| `building` | 楼宇 (关联 project) | 1 |
| `device` | 设备 (关联 building) | 20 |
| `device_properties` | 设备属性 | 8 |
| `register` | 寄存器 (关联 property) | 11 |
| `users` | 用户 (关联 agent + role) | 3 |
| `role` | 角色 | 8 |
| `permission` | 权限点 | 37 |
| `role_permission` | 角色-权限关联 | 167 |
| `alarm_rule` | 告警规则 | 5 |
| `alarm_log` | 告警日志 | 4 |
| `control_record` | 控制记录 | 3 |
| `startup_plan` | 启停计划 | 2 |
| `startup_step` | 启停步骤 | 3 |
| `startup_execution` | 启停执行记录 | 1 |
| `startup_step_log` | 启停步骤日志 | 1 |
| `device_telemetry` | 设备遥测 (TimescaleDB) | 0 |
| `dashboard_config` | 仪表盘配置 | 6 |
| `weather_cache` | 天气缓存 (30/60min TTL) | 动态 |
| `schema_migrations` | 迁移版本追踪 (自动) | 5 |
| `electricity_price` | 分时电价配置 | — |

> 另有 7 张预留表 (`gateway_config`/`interlock_config`/`auto_control`/`fft_baseline`/`agent_config`/`user_project` 等) 已建表但暂无数据。

### 角色体系 (8 级)

| 角色 | Code | Level | 权限范围 |
|---|---|---|---|
| 超级管理员 | super_admin | 0 | 全部 (自动绕过 RBAC) |
| 平台管理员 | admin | 10 | 除系统配置外的全部 |
| 代理商管理员 | agent_admin | 20 | 项目/楼宇/设备/监控/报表 |
| 代理商运维 | agent_operator | 30 | 设备操作 + 监控 |
| 代理商查看者 | agent_viewer | 40 | 只读 |
| 项目管理员 | project_admin | 50 | 项目范围 CRUD |
| 项目运维 | project_operator | 60 | 设备级操作 |
| 项目查看者 | project_viewer | 70 | 只读 |

---

## API 端点 (56 条路由)

### 认证
```
POST /api/v1/auth/login          # 登录 (含速率限制 10次/分钟/IP)
GET  /api/v1/auth/me              # 当前用户信息
```

### 业务 CRUD
```
GET|POST             /api/v1/projects
GET|PUT|DELETE       /api/v1/projects/{id}
GET|POST             /api/v1/buildings
GET|PUT|DELETE       /api/v1/buildings/{id}
GET|POST             /api/v1/devices
GET|PUT|DELETE       /api/v1/devices/{id}
POST                 /api/v1/devices/{id}/control
GET|POST             /api/v1/properties
GET|PUT|DELETE       /api/v1/properties/{id}
GET|POST             /api/v1/registers
GET|PUT|DELETE       /api/v1/registers/{id}
GET|POST             /api/v1/alarm-rules
PUT|DELETE           /api/v1/alarm-rules/{id}
GET                  /api/v1/alarm-logs
POST                 /api/v1/alarm-logs/{id}/ack
GET|POST             /api/v1/startup-plans
PUT|DELETE           /api/v1/startup-plans/{id}
POST                 /api/v1/startup-plans/{id}/execute
GET                  /api/v1/startup-executions/{id}
POST                 /api/v1/startup-executions/{id}/stop
```

### 遥测 & 日志
```
GET /api/v1/telemetry/realtime    # 实时数据 (支持 device_id 筛选/全部)
GET /api/v1/telemetry/history     # 历史数据
GET /api/v1/telemetry/stats       # 统计 (单设备指标 / 系统级在线率)
GET /api/v1/logs/telemetry        # 遥测日志 (支持 CSV 导出)
GET /api/v1/logs/controls         # 控制记录 (按 device_id JOIN 过滤)
GET /api/v1/logs/stats            # 统计日志
```

### 天气 (wttr.in + DB 缓存)
```
GET /api/v1/weather/cities         # 城市搜索 (支持 ?q=)
GET /api/v1/weather/cities/{id}    # 单个城市
GET /api/v1/weather/provinces      # 省→市树形列表 (级联选择器)
GET /api/v1/weather/now            # 实时天气 (?city_id= 或 ?city_name=)
GET /api/v1/weather/project        # 项目天气 (?project_id=)
```

### 智能分析
```
GET /api/v1/intelligence/full             # 全部分析 (能效+预测+建议+摘要)
GET /api/v1/intelligence/efficiency       # 设备能效分析
GET /api/v1/intelligence/forecast         # 24h 负荷预测
GET /api/v1/intelligence/recommendations  # 设定点优化建议
GET /api/v1/intelligence/strategies       # 协同控制策略 (联动+泵优化+电价+轮换)
GET /api/v1/intelligence/price-config     # 电价配置查询
PUT /api/v1/intelligence/price-config     # 电价配置保存
```

### 权限管理
```
GET|POST             /api/v1/users
PUT                  /api/v1/users/{id}
POST                 /api/v1/users/{id}/reset-password
DELETE               /api/v1/users/{id}
GET|POST             /api/v1/agents
PUT|DELETE           /api/v1/agents/{id}
GET                  /api/v1/roles
GET                  /api/v1/permissions
GET                  /api/v1/roles/{id}/permissions
PUT                  /api/v1/roles/{id}/permissions
```

### 导出 & 系统
```
GET /api/v1/export/telemetry      # 遥测 CSV 导出
GET /api/v1/export/controls       # 控制记录 CSV 导出
GET /api/v1/system/info           # 系统信息 (DB 版本)
GET /api/v1/system/db-stats       # 数据库统计 (大小/连接数/表数/行数)
```

### 其他
```
GET /api/v1/dashboard             # 仪表盘配置 (含动态指标)
PUT /api/v1/dashboard             # 更新仪表盘配置
GET /api/v1/gateways              # 网关列表
GET /api/health                   # 健康检查
```

---

## Web 后台路由

```
/login          → 登录页
/               → 系统概览仪表盘 — 在线设备/今日告警/运行天数 + 天气卡片
/users          → 用户管理 (CRUD + 编辑 + 角色/代理商筛选)
/agents         → 代理商管理 (CRUD + 下辖用户/项目数，项目可下钻)
/permissions    → 权限管理 — 37/37 全覆盖
/projects       → 项目管理 — 城市级联选择 + 地址/行政区划码自动填充 + 楼宇数下钻
/buildings      → 楼宇管理 — 项目筛选 + 设备数下钻 + 启停跳转
/devices        → 设备管理 — 所属项目/楼宇名列 + 属性/日志跳转
/properties     → 属性配置 — 所属设备/楼宇/项目列 + 寄存器数列
/registers      → 寄存器 — 所属属性/设备列 + 四级级联下拉
/alarms         → 告警管理 — 规则 + 日志表格 + URL 参数联动
/startup-plans  → 启停配置 — 计划 + 步骤 + 执行 + URL 参数
/logs           → 日志管理 — 三 Tab (设备数据/操作日志/统计数据) + CSV 导出
/multi-agent    → 智能分析 — 双 Tab (智能分析|协同控制) 能效表+负荷图+建议+联动+电价+轮换
```

### 跨模块 URL 参数联动

```
Agents(?agent_id=X) ──下辖项目──→ Projects(?agent_id=X)
    ↑ 用户数
    └── Users (agent_id 筛选)

Projects ──楼宇数──→ Buildings(?project_id=X) ──设备数+启停──→ Devices(?building_id=X) / StartupPlans(?building_id=X)
                         ↑ 项目筛选下拉                             ↑ 属性/日志跳转
                                                                   ├── Properties(?device_id=X) → Registers(?property_id=X)
                                                                   ├── Logs(?device_id=X)
                                                                   └── Alarms(?device_id=X)
```

---

## 工程指标

| 维度 | 数据 |
|------|------|
| Go 源文件 | 54 个 |
| 测试文件 | 10 个 (覆盖率 18.5%) |
| 测试用例 | 55+ (0 失败) |
| 测试覆盖包 | 11/18 |
| Web TSX | 18 个 |
| 小程序 Vue | 25 个 |
| API 路由 | 56 条 |
| 数据库表 | 23 张 |
| 内置城市 | 300+ (34 省/自治区/直辖市) |
| go vet | clean ✅ |
| go build | OK ✅ |

---

## 测试覆盖

| 包 | 测试文件 | 覆盖范围 |
|---|---------|---------|
| `service/intelligence` | ✅ | 湿球温度、负荷预测、精度取舍、Demo 数据、优先级 |
| `service/auth` | ✅ | bcrypt 哈希、JWT 签发/验证/过期/错误密钥 |
| `service/alarm` | ✅ | gt/ge/lt/le/eq 条件判断 + benchmark |
| `service/external/weather` | ✅ | wttr.in JSON 解析、中文城市、缓存/超时常量 |
| `api/middleware` | ✅ | 限流器 (单IP/多IP/并发)、CORS 头+预检 |
| `api/handler` | ✅ | pathLast、queryInt、writeJSON 工具函数 |
| `config` | ✅ | 环境变量加载、DSN 生成、fallback |
| `repository/postgres` | ✅ | Building/Device/Register/Project/Admin CRUD 模拟 |
| `service/orchestrator` | ✅ | 启停步骤条件判断 + 联锁检查 |

---

## 架构亮点

### 1. 自动迁移 (Migration Runner)
- `internal/service/migration/` — Go embed 嵌入 SQL 文件
- 启动时自动检测已应用版本，智能跳过
- `schema_migrations` 表追踪版本，事务执行
- 新增迁移只需放 `sql/` 目录

### 2. 外部 API 模块
- `internal/service/external/weather/` — wttr.in 免费天气
- 30/60 分钟 DB 缓存，API 失败自动回退历史数据
- 城市级联选择器 (省→市)，300+ 城市，自动填充地址/行政区划码
- 预留扩展：后续其他外部 API (地图、电价等) 也放此目录

### 3. 多智能体分析引擎
- **Phase 1** — 设备能效分析 + 24h 负荷预测 + 设定点推荐
- **Phase 2** — 协同控制规则引擎 (联动策略 + 泵优化 + 电价 + 轮换)
- 湿球温度计算 (Stull 公式)
- 办公楼负荷曲线 × 温度修正
- 分时电价策略 (默认深圳商业电价)

### 4. 代码质量
- 错误处理：scan 失败统一 slog.Warn + continue
- 并发安全：rate limiter sync.Mutex
- 幂等设计：迁移 IF NOT EXISTS / ON CONFLICT DO NOTHING
- 前后端分离：Ant Design Cascader + REST API 树形数据

---

## 网关接入

| 网关类型 | 端口 | 标识方式 | 长度 | 说明 |
|----------|------|----------|------|------|
| Custom (自定义) | 8081 | MAC 地址 hex | 12 位 | 私有协议 0x68/0x16 帧头，MAC 自动注册 |
| DTU (透传) | 502 | GWID 注册 / IP 匹配 | 可变 | 原始 Modbus RTU over TCP，有人 G770 等 |

`device.gateway_imei` (VARCHAR(20)) 关联设备到网关。Custom 填 MAC、DTU 填 IMEI(15位)。

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
| `XMECO_WEATHER_API_KEY` | (空) | 和风天气 Key (预留，当前用 wttr.in) |
