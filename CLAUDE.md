# XMECO 多智能体能效节能系统 — 项目全貌与 AI 协作规范

> 本文档是项目唯一的技术入口，包含：项目概述、技术栈基线、架构约定、编码规范、AI 协作规则、测试要求、项目状态追踪。
> **对项目做任何修改前，必须先通读本文档。**

---

## 1. 项目概述

XMECO（熊猫智控）是一套面向**中央空调水冷系统**的 AIOT 能效管理平台，通过 Modbus/TCP 实时采集冷水主机、冷却塔、水泵、电表等设备数据，进行时序存储、告警评估、能效分析和智能联动控制。

| 模块 | 技术栈 | 端口 | 说明 |
|---|---|---|---|
| 后端 API | Go 1.25 + PostgreSQL | 9090 | RESTful API，JWT 认证，RBAC 权限 |
| Web 大屏 | React 19 + Ant Design + Vite | 3001 | 深色全屏大屏，设备拓扑图 + 实时控制 |
| Web 管理后台 | React 19 + Ant Design + Vite | 3000 | 管理员后台，15 个功能页面 |
| 小程序 | uni-app (Vue 3) | 5173 | 移动端运维，支持 H5/微信 |

### 快速启动

```bash
# 一键启动（双击）
d:\py\xmeco-new\start.bat

# 手动启动
docker start xmeco-pg
.\server.exe
cd web\admin && npm run dev          # 管理后台 :3000
cd web\admin && npm run dev:screen   # 大屏 :3001
```

| 服务 | 地址 | 登录 |
|---|---|---|
| Web 大屏 | http://localhost:3001 | admin / admin123 |
| Web 管理后台 | http://localhost:3000 | admin / admin123 |
| 小程序 H5 | http://localhost:5173 | admin / admin123 |

---

## 2. 技术栈与基线锁定

### 后端
| 类别 | 技术 | 版本 |
|---|---|---|
| 语言 | Go | 1.25 |
| 路由 | `net/http` (标准库 ServeMux) | Go 1.22+ |
| 数据库驱动 | `github.com/jackc/pgx/v5` | v5.10.0 |
| 数据库 | PostgreSQL | 15+ |
| 认证 | `github.com/golang-jwt/jwt/v5` | v5.3.1 |
| 密码 | `golang.org/x/crypto` (bcrypt) | v0.53.0 |
| 日志 | `log/slog` (标准库) | Go 1.21+ |
| 测试 Mock | `github.com/pashagolub/pgxmock/v4` | v4.9.0 |

### Web 管理后台 + 大屏
| 类别 | 技术 | 版本 |
|---|---|---|
| 框架 | React | 19.2.6 |
| UI 库 | Ant Design (`antd`) | 6.4.4 |
| 路由 | `react-router-dom` | 7.18.0 |
| HTTP | `axios` | 1.18.0 |
| 构建 | Vite | 8.0.12 |
| CSS | TailwindCSS v4 | 4.3.1 |
| 语言 | TypeScript | ~6.0.2 |

### 小程序
| 类别 | 技术 | 版本 |
|---|---|---|
| 框架 | uni-app (Vue 3) | 3.0.0 |
| 语言 | TypeScript | 4.9.4 |
| 构建 | Vite | 5.2.8 |

### ⚠️ 版本锁定规则

**以上版本号是项目基线，严禁擅自升级。** 包括但不限于：`go.mod`、`package.json`、Go 编译器、Node.js、PostgreSQL 版本。

**唯一允许升级的情形**：用户明确给出"升级 X 到 Y.Z.W"的具体指令。

**原因**：版本变更引入未知兼容性问题，在物联网系统中可能导致硬件通信中断。

---

## 3. 目录结构

```
xmeco-new/
├── start.bat                          # 一键启动
├── cmd/server/main.go                 # 后端入口：配置→DB→迁移→服务→95条路由
├── internal/
│   ├── api/
│   │   ├── handler/                   # HTTP Handler — 参数解析→调用Service/Repo→序列化响应
│   │   │   ├── models.go              # M()/pathID/queryInt/writeJSON 公共工具 + GatewayManager/HardwareDispatcher 接口
│   │   │   ├── admin.go               # 用户/代理商/角色/权限 CRUD
│   │   │   ├── alarm.go               # 告警规则 + 日志
│   │   │   ├── auth.go                # 登录 + Me
│   │   │   ├── dashboard.go           # 大屏 ScreenData 聚合端点
│   │   │   ├── intelligence.go        # 智能分析 + 电能质量 + 电表
│   │   │   ├── log.go                 # 遥测/控制日志 + CSV 导出
│   │   │   ├── maintenance.go         # 维保记录 CRUD
│   │   │   ├── project.go             # 项目 CRUD + 用户分配
│   │   │   ├── startup.go             # 启停计划 + 定时任务
│   │   │   ├── telemetry.go           # 实时/历史遥测
│   │   │   └── weather.go             # 天气查询
│   │   └── middleware/
│   │       ├── auth.go                # JWT 认证
│   │       ├── bodylimit.go           # 1MB 请求体限制
│   │       ├── cors.go                # CORS 跨域
│   │       ├── ratelimit.go           # 登录限流
│   │       └── rbac.go                # RequirePermission 权限
│   ├── config/config.go               # 环境变量配置（11个配置项）
│   ├── domain/models.go               # 领域结构体（Building/Device/Register等）
│   ├── gateway/                       # 硬件通信层
│   │   ├── manager.go                 # 双协议监听(Custom+DTU) + 设备注册 + pollLoop
│   │   ├── modbus/modbus.go           # Modbus RTU 帧构建/解析/CRC16
│   │   └── transport/                 # Transport 接口 + Custom(0x68协议) + Transparent(DTU)
│   ├── repository/postgres/           # 数据访问层 — DBTX 接口 + pgxmock 测试
│   └── service/
│       ├── alarm/engine.go            # 告警引擎 (gt/ge/lt/le/eq/range) + 离线告警
│       ├── auth/                      # bcrypt 认证 + JWT + RBAC
│       ├── external/weather/          # wttr.in + DB 缓存 60min
│       ├── intelligence/              # 多智能体引擎（能效/预测/联动/电价/轮换/电能质量）
│       ├── migration/                 # Go embed 自动迁移 (16个版本)
│       ├── orchestrator/              # 启停编排 + 联锁检查
│       └── telemetry/poller.go        # Modbus 轮询 → 解码 → Batch 写入 → 告警
│   ├── safego/
│   │   └── safego.go                   # safego.Go — goroutine panic 恢复公共工具
├── web/admin/
│   ├── src/
│   │   ├── App.tsx                    # 管理后台路由（/login → 14页面）
│   │   ├── api/client.ts              # Axios 客户端 (JWT拦截 + 401自动登出)
│   │   ├── pages/                     # 15个页面组件
│   │   └── layouts/Main.tsx           # 侧边栏布局
│   └── screen-src/                    # 大屏独立入口（端口3001）
│       ├── ScreenApp.tsx
│       └── vite.config.ts
└── miniapp/                           # uni-app 小程序 (Vue 3)
    └── src/
        ├── api/client.ts              # API 客户端 (uni.request + JWT)
        ├── pages/                     # 7个页面
        └── stores/                    # Pinia 状态管理
```

---

## 4. 架构约定

### 分层架构（单体，严格四层）

```
HTTP Request
    │
    ▼
┌──────────────┐
│  Middleware    │  cors → auth(JWT) → rbac(permission)
└──────┬───────┘
       ▼
┌──────────────┐
│  Handler       │  参数解析 → 调用 Service/Repository → 序列化 JSON
└──────┬───────┘
       ▼
┌──────────────┐
│  Service       │  业务逻辑：告警评估、智能分析、天气缓存、启停编排
└──────┬───────┘
       ▼
┌──────────────┐
│  Repository    │  SQL 封装，DBTX 接口解耦（接受 *pgxpool.Pool 或 pgx.Tx）
└──────┬───────┘
       ▼
┌──────────────┐
│  PostgreSQL    │  pgxpool 连接池
└──────────────┘
```

### 横向层（独立于 HTTP 请求）
- **Gateway Layer**：Manager → TCP 监听 → 设备注册 → pollLoop → Modbus 读写 → 遥测落库
- **Background Tasks**（3 个常驻 goroutine，均通过 `safego.Go` 启动带 panic 恢复）：数据清理(24h)、离线检测(1min)、定时启停(1min)

### 数据流
```
硬件网关 ──TCP──► Manager ──Transport──► Poller.PollDevice()
                                              │
                                     ┌────────┴────────┐
                                     │ Modbus 读取 →    │
                                     │ decodeVal 解码 → │
                                     │ Batch 写入 DB →  │
                                     │ 标记在线 →       │
                                     │ 告警评估          │
                                     └────────┬────────┘
                                              ▼
                              device_telemetry / alarm_log / device.status
```

---

## 5. 编码规范

### 5.1 Go 后端

| 规范项 | 约定 |
|---|---|
| **注释语言** | ⚠️ 所有注释使用中文 |
| **日志** | 统一 `slog.Info/Warn/Error`，错误必须含 key=value 上下文 |
| **错误处理** | ⚠️ 严禁 `_` 丢弃 error。仅 `queryInt` 等辅助函数允许丢弃 `Atoi` 错误 |
| **空值安全** | SQL 可空列用 `COALESCE()` 包裹；`*T` 指针取值前必须判 nil |
| **JSON 标签** | 必须标注 `json:"snake_case"` |
| **SQL 参数** | 严格 `$1, $2, ...` 占位符，**禁止字符串拼接用户输入**。动态参数（如 interval）必须白名单校验 |
| **GetByID** | ⚠️ 查不到记录返回 `nil, nil`（不是 error）。Handler 先判 `err != nil`→500，再判 `obj == nil`→404 |
| **DBTX 接口** | Repo 构造函数接受 `DBTX` 接口（含 `*pgxpool.Pool` 和 `pgx.Tx`），支持事务复用 |

### 5.2 命名规范（Go）

| 类别 | 规则 | 示例 |
|---|---|---|
| 包 | 小写、单数、无分隔符 | `handler` `telemetry` |
| Go 文件名 | 小写+下划线 | `rate_limit.go` `engine_test.go` |
| 结构体 | PascalCase | `BuildingHandler` `AlarmEngine` |
| 字段 | PascalCase（导出）/camelCase（私有），JSON标签snake_case | `DeviceID int \`json:"device_id"\`` |
| 函数/方法 | PascalCase（导出）/camelCase（私有） | `GetByID()` `decodeVal()` |
| 变量 | camelCase，简短有意义 | `deviceID` `gwMgr` |
| SQL 迁移文件 | `NNN_英文描述.sql` | `012_add_device_snapshot.sql` |
| 测试文件 | `xxx_test.go` | `auth_test.go` |

### 5.3 TypeScript / React 前端

| 规范项 | 约定 |
|---|---|
| 缩进 | 2 空格，单引号，无分号 |
| 组件 | 函数式组件 + Hooks，文件 `PascalCase.tsx` |
| API 调用 | 统一使用 `src/api/client.ts` 的 `api` 实例 |
| UI | 强制使用 `antd` 组件 |
| 状态 | `useState/useEffect`，管理后台无全局状态库 |

### 5.4 小程序 (Vue 3)

| 规范项 | 约定 |
|---|---|
| API | 组合式 API (`<script setup lang="ts">`) |
| 状态 | Pinia (`src/stores/`) |
| 组件 | PascalCase 命名 |

---

## 6. 常用命令

```bash
# === Go 后端 ===
cd d:\py\xmeco-new
go build ./...                          # 构建
go test ./internal/...                  # 全部测试
go test -v ./internal/repository/postgres/  # 单包测试
go vet ./...                            # 静态检查

# === Web 管理后台 ===
cd d:\py\xmeco-new\web\admin
npm run dev                             # 管理后台 :3000
npm run dev:screen                      # 大屏 :3001
npm run build                           # 编译检查
npm run lint                            # 代码检查

# === 小程序 ===
cd d:\py\xmeco-new\miniapp
npm run dev:h5                          # H5 :5173
npm run dev:mp-weixin                   # 微信小程序
npm run type-check                      # 类型检查
```

---

## 7. 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `XMECO_DB_HOST` | localhost | 数据库主机 |
| `XMECO_DB_PORT` | 5432 | 数据库端口 |
| `XMECO_DB_USER` | postgres | 数据库用户 |
| `XMECO_DB_PASSWORD` | xmeco123 | 数据库密码 |
| `XMECO_DB_NAME` | xmeco | 数据库名 |
| `XMECO_DB_SSLMODE` | disable | SSL 模式 |
| `XMECO_SERVER_PORT` | 9090 | HTTP 端口 |
| `XMECO_JWT_SECRET` | ⚠️ 无默认值 | JWT 签名密钥 — **生产环境缺省将拒绝启动 (`os.Exit(1)`)** |
| `XMECO_ALLOWED_ORIGINS` | `*` | CORS 白名单（逗号分隔域名） |
| `XMECO_RETENTION_DAYS` | 730 | 遥测保留天数（0=不清理） |
| `XMECO_POLL_INTERVAL_SEC` | 3 | 设备轮询间隔秒数 |
| `XMECO_TRUSTED_PROXY` | 空 | 可信代理 CIDR |

⚠️ 日志中禁止输出含密码的 DSN，使用 `config.MaskedDSN()` 脱敏。不存在配置文件，所有配置通过环境变量注入。

---

## 8. 数据库与迁移

### 核心表（24 张）

| 表名 | 说明 | 表名 | 说明 |
|---|---|---|---|
| `agent` | 代理商 | `alarm_rule` | 告警规则 |
| `project` | 项目 | `alarm_log` | 告警日志 |
| `project_user` | 项目-用户分配 | `startup_plan` | 启停计划 |
| `city` | 城市 (300+) | `startup_step` | 启停步骤 |
| `building` | 楼宇 | `scheduled_task` | 定时任务 |
| `device` | 设备 | `device_telemetry` | 遥测时序数据 |
| `device_properties` | 设备属性 | `control_record` | 控制记录 |
| `register` | 寄存器配置 | `dashboard_config` | 仪表盘配置 |
| `users` | 用户 | `weather_cache` | 天气缓存 |
| `role` | 角色 (8级) | `sensor_config` | 传感器配置 |
| `permission` | 权限点 (37个) | `maintenance_record` | 维保记录 |
| `role_permission` | 角色-权限关联 | `schema_migrations` | 迁移版本追踪 |

### 迁移规范

| ⚠️ 规则 | 说明 |
|---|---|
| **文件命名** | `NNN_英文下划线描述.sql`（如 `012_add_sensor_config.sql`） |
| **严禁修改已有迁移** | 已合入的迁移文件不可修改，变更必须通过新迁移 |
| **严禁删除迁移** | 删除会导致 `schema_migrations` 与实际 Schema 不一致 |
| **必须幂等** | 使用 `IF NOT EXISTS`、`ON CONFLICT DO NOTHING`、`ADD COLUMN IF NOT EXISTS` |
| **禁用 DROP** | 迁移中不要 `DROP TABLE/COLUMN`，先在代码中停止引用再单独处理 |
| **执行机制** | Go `embed` 嵌入 SQL → 启动时自动按序执行 → 事务包装 → 跳过已应用版本 |

---

## 9. API 端点（95 条路由）

| 分类 | 路由前缀 | 说明 |
|---|---|---|
| 大屏 | `GET /api/v1/screen/data` | 一站式聚合（项目/建筑/设备/天气/任务/告警/能耗） |
| 认证 | `POST .../auth/login` `GET .../auth/me` | JWT 登录 + 当前用户 |
| 业务 CRUD | `/projects` `/buildings` `/devices` `/properties` `/registers` | 标准 RESTful（含设备控制 `POST .../devices/{id}/control`、传感器 `GET .../devices/{id}/sensor-data` `PUT .../devices/{id}/sensor-config`） |
| 告警 | `/alarm-rules` `/alarm-logs` | 规则 CRUD + 日志查询 + 确认(ack) |
| 维保 | `/maintenance-records` | 维保记录 CRUD |
| 启停 | `/startup-plans` `/startup-executions` | 计划 CRUD + 执行/监控/停止 |
| 定时任务 | `/scheduled-tasks` | once/daily/weekly 调度 |
| 遥测 | `/telemetry/realtime` `/telemetry/history` `/telemetry/stats` | 实时/历史/统计 |
| 日志 | `/logs/telemetry` `/logs/controls` `/logs/stats` | 支持 interval=raw~year |
| 天气 | `/weather/cities` `/weather/now` `/weather/project` | 城市/省份/当前/项目天气 |
| 智能分析 | `/intelligence/full` `/intelligence/efficiency` `/intelligence/forecast` `/intelligence/recommendations` `/intelligence/strategies` `/intelligence/power-quality` `/intelligence/meter-devices` | 多智能体引擎全部端点 |
| 权限管理 | `/users` `/agents` `/roles` `/permissions` | 用户/代理商/角色/权限 CRUD + 密码重置 |
| 导出 | `/export/telemetry` `/export/controls` | CSV 导出 |
| 系统 | `/system/info` `/system/db-stats` `/dashboard` `/gateways` `/health` | 系统信息 + 仪表盘 + 网关 |

---

## 10. AI 协作规则

### 10.1 只读文件（不要修改）

| 文件/目录 | 原因 |
|---|---|
| `go.sum` | 由 `go mod tidy` 自动生成 |
| `web/admin/dist/` | 构建产物 |
| 已有迁移 SQL 文件 (`internal/service/migration/sql/0*.sql`) | 历史版本不可变 |

### 10.2 高风险模块（修改需格外小心）

| 模块 | 风险点 |
|---|---|
| `gateway/transport/custom.go` | 自定义协议帧解析，字节偏移错误导致网关通信中断 |
| `gateway/transport/transparent.go` | DTU 透传，Drain 循环有上限保护(10次)，注意互斥锁释放 |
| `service/telemetry/poller.go` | 核心采集链路，decodeVal 涉及多字节序/掩码/倍率 |
| `service/alarm/engine.go` | 告警去重依赖部分唯一索引，修改 ON CONFLICT 语句注意索引条件 |
| `config/config.go` | JWT 密钥缺省触发 `os.Exit(1)`，测试必须 `os.Setenv` |
| `cmd/server/main.go` | 路由注册 + 3 goroutine（均通过 `safego.Go` 启动），新增路由须注册权限码 |

### 10.3 新增功能标准步骤

**新增 CRUD 实体：**
```
1. domain/models.go          → 添加 struct
2. repository/postgres/      → 新建 repo.go，实现 List/GetByID/Create/Update/Delete
3. api/handler/              → 新建 handler.go，参数解析→调用 repo→ok/created/notFound
4. cmd/server/main.go        → 实例化 repo+handler，registerRoutes 中注册路由+权限码
5. migration/sql/            → 新建 NNN_xxx.sql，CREATE TABLE IF NOT EXISTS
```

**新增 Service 逻辑：**
```
1. service/xxx/              → 新建目录，实现 Service struct + New() 构造函数
2. 构造函数接受 *pgxpool.Pool 和必要配置
3. cmd/server/main.go        → 实例化并注入到需要的 Handler
```

### 10.4 依赖方向（单向不可逆）

```
Handler ──────► Service ──────► Repository ──────► PostgreSQL
  │                │                │
  └── Domain ◄─────┴────────────────┘  (被所有层引用，不引用任何层)
```

| ✅ 允许 | ❌ 禁止 |
|---|---|
| Handler → Service | Repository → Service |
| Handler → Repository（历史遗留，新功能禁止） | Repository → Handler |
| Service → Repository | Handler 直接持有 `*pgxpool.Pool`（新功能） |
| Service → `*pgxpool.Pool`（复杂 SQL 如告警引擎） | 循环 import |

**判断标准**：如果需要 import 更高层的包，说明设计有问题，应提取接口或重新组织。

### 10.5 横切关注点（每个 Handler 必须满足）

| 关注点 | 实现 |
|---|---|
| 认证 | 所有 `/api/v1/*` 走 JWT 认证（登录和 health 除外） |
| 授权 | 非公开路由用 `withPerm(authSvc, "perm.code", handler)` 包装 |
| 限流 | 登录接口受 `rateLimiter.LimitLogin()` 保护 |
| CORS | 顶层 mux 经过 CORS 中间件 |
| 日志 | 所有错误路径 `slog.Warn/Error` |
| 超时 | 任何 TCP/HTTP 外部调用必须有超时 |
| panic恢复 | 所有后台 goroutine 必须通过 `safego.Go(name, ctx, fn)` 启动 |

### 10.6 禁止事项

| ❌ 禁止 | 正确做法 |
|---|---|
| 用 `_` 丢弃 error | `slog.Warn("msg", "err", err)` |
| `fmt.Println` / `log.Println` | `slog.Info/Warn/Error` |
| SQL 字符串拼接用户输入 | `$1, $2` 占位符 + 白名单校验 |
| `*T` 指针不判 nil 直接解引用 | `if p != nil { use(*p) }` |
| HTTP Handler 中 `panic()` | 返回 error + `serverErr(w, err)` |
| 循环中逐行 INSERT | 使用 `pgx.Batch` 批量写入 |
| CORS `*` 且不校验 Origin（生产） | 设置具体域名白名单 |
| JWT 密钥使用默认值 | 生产环境设置 `XMECO_JWT_SECRET` |
| 密码重置跳过旧密码验证 | 必须验证旧密码 |
| interval 参数直接拼入 SQL | 白名单校验 |
| 新增 Repo/Service 代码不写测试 | 参照第 11 节测试规范 |
| 修改已有迁移 SQL 文件 | 新增迁移文件 |

---

## 11. 测试规范

### 11.1 硬性要求

| 模块类型 | 要求 | 测试方式 |
|---|---|---|
| **Repository**（新建或新增方法） | 🔴 必须 | `pgxmock` 表驱动 |
| **Gateway/Transport**（协议解析） | 🔴 必须 | 纯字节数组单元测试 |
| **Service**（含业务逻辑的方法） | 🟡 必须 | `pgxmock` + 表驱动 |
| **Handler**（新建端点） | 🟡 必须 | `httptest.NewRecorder` + mock |
| **工具函数**（decodeVal等） | 🟢 强烈建议 | 表驱动 |
| **前端页面** | 🟢 暂不强制 | — |

### 11.2 Repository 测试模板

```go
func TestFooRepo_List(t *testing.T) {
    mock, err := pgxmock.NewPool()
    if err != nil { t.Fatal(err) }
    defer mock.Close()

    rows := mock.NewRows([]string{"id", "name"}).AddRow(1, "test")
    mock.ExpectQuery(`SELECT id, name FROM foo`).WillReturnRows(rows)

    repo := NewFooRepo(mock)
    list, err := repo.List(context.Background(), 0)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if len(list) != 1 { t.Errorf("want 1, got %d", len(list)) }

    if err := mock.ExpectationsWereMet(); err != nil { t.Errorf("unmet: %v", err) }
}
```

### 11.3 不需要测试的情况

- `cmd/server/main.go` 路由注册和 goroutine 启动（集成逻辑）
- 纯数据定义文件（`domain/models.go` struct）
- SQL 迁移文件本身
- 前端纯展示组件（无交互）

---

## 12. 代码模式速查

### Handler 标准模式（带 Repository）

```go
type FooHandler struct{ repo *postgres.FooRepo }
func NewFooHandler(r *postgres.FooRepo) *FooHandler { return &FooHandler{r} }

func (h *FooHandler) List(w http.ResponseWriter, r *http.Request) {
    list, err := h.repo.List(r.Context(), queryInt(r, "parent_id"))
    if err != nil { serverErr(w, err); return }
    if list == nil { list = []domain.Foo{} }
    ok(w, list)
}

func (h *FooHandler) Get(w http.ResponseWriter, r *http.Request) {
    obj, err := h.repo.GetByID(r.Context(), pathID(r))
    if err != nil { serverErr(w, err); return }
    if obj == nil { notFound(w, "资源不存在"); return }
    ok(w, obj)
}
```

### Repository 标准模式（含 GetByID 约定）

```go
type FooRepo struct{ pool DBTX }  // DBTX = *pgxpool.Pool | pgx.Tx
func NewFooRepo(pool DBTX) *FooRepo { return &FooRepo{pool} }

func (r *FooRepo) GetByID(ctx context.Context, id int) (*domain.Foo, error) {
    var f domain.Foo
    err := r.pool.QueryRow(ctx, "SELECT id, name FROM foo WHERE id=$1", id).Scan(&f.ID, &f.Name)
    if errors.Is(err, pgx.ErrNoRows) { return nil, nil }  // ⚠️ 未找到返回 nil, nil
    return &f, err
}
```

### 路由注册模式

```go
func registerRoutes(db *postgres.DB, rl *middleware.RateLimiter, authSvc *auth.Service, ...) *http.ServeMux {
    p := auth.Perm
    wp := func(code string, next http.HandlerFunc) http.HandlerFunc {
        return withPerm(authSvc, code, next)
    }

    mux := http.NewServeMux()
    protected := http.NewServeMux()
    protected.HandleFunc("GET /api/v1/foos", wp(p.FooView, h.List))
    protected.HandleFunc("POST /api/v1/foos", wp(p.FooCreate, h.Create))
    // ...
    mux.Handle("/api/v1/", middleware.BodyLimit(1<<20)(middleware.AuthMiddleware(authSvc)(protected)))
    return mux
}
```

⚠️ **Handler 与 Repository 不可混用。** 如果创建了 Repo，Handler 必须通过 Repo 访问数据库，不要直接持有 `*pgxpool.Pool`。当前 `AlarmHandler/TelemetryHandler/LogHandler/DashboardHandler/StartupHandler/MaintenanceHandler` 直接 SQL 是历史遗留（有 plan 但未执行拆离）。

---

## 13. 缓存策略

### 当前状态

项目**无 Redis 或内存缓存库**。唯一缓存是天气服务的数据库表缓存：

| 缓存对象 | 存储 | TTL | 降级策略 |
|---|---|---|---|
| 天气数据 | `weather_cache` 表 | 60min | API 失败时回退到最新记录（不限过期） |

### 何时引入缓存

| 场景 | 方案 |
|---|---|
| 读多写少、允许延迟 | 数据库表缓存（仿 `weather_cache`，建 `_cache` 表） |
| 大屏 `/screen/data` 高频访问 | `sync.Map` 内存缓存，TTL 3-5 秒 |
| 告警去重 | 已用 PostgreSQL 部分唯一索引，无需额外缓存 |

### 不要缓存

- 实时遥测数据（poller 高频写入，缓存导致滞后）
- 设备控制操作（必须实时下发）
- 告警日志查询（需看到最新）
- 任何强一致性写入路径

---

## 14. 项目状态

> 最后更新：2026-06-28

### 已完成
- 后端 95 条路由 + 24 张表 + 16 个迁移版本，安全加固（CORS/JWT/SQL注入/空指针/密码脱敏）
- 深度代码审查 + 修复：SQL 确定性删除、写功能码告警、日期参数校验、queryInt/pathID 语义修正
- 架构改进：`safego.Go` 公共 goroutine 工具、`HardwareDispatcher` 接口解耦 DeviceHandler-StartupHandler
- 智能算法常量化：`copGainPerDegreeC`/`saveKWPerDegreeC`/`minApproachTemp` 替代魔法数字
- 清理：删除冗余文件（AGENTS.md、旧报告、日志、二进制），完善 .gitignore
- Web 大屏：深色全屏 + 设备拓扑 + 实时数据 + 天气/告警/能耗 + 传感器+历史图表
- Web 管理后台：15 个页面框架，登录+CRUD 可用
- 小程序：7 个页面（登录/设备/详情/告警/历史/我的）
- 测试：10 个测试文件，~170 用例全部通过

### 当前阻碍

| 编号 | 阻碍 | 状态 |
|---|---|---|
| B-01 | 大屏 5s 轮询对 DB 压力（每次 8+ SQL） | 待引入 WebSocket 或缓存 |
| B-02 | 后端需手动重启，无热重载 | 可用 `air` 工具 |
| B-03 | DataCenter N+1 请求（每设备单独请求 /properties） | 待新增批量接口 |

### 下一步

| 优先级 | 任务 |
|---|---|
| P0 | 大屏维保中心/任务中心 Tab 实现 |
| P1 | WebSocket 实时推送（替换 5s 轮询） |
| P1 | 告警通知渠道（短信/公众号/邮件） |
| P1 | 设备控制闭环（Modbus 写后读回确认） |
| P1 | Handler 直接 SQL 拆离到 Repository（AlarmHandler 等 6 个） |

---

## 15. 规则优先级

```
1. 安全规则            ← 最高优先级，不可妥协
2. 禁止事项（10.6）
3. 依赖方向（10.4）
4. 横切关注点（10.5）
5. 编码规范（第 5 节）
6. 命名规范（5.2）
7. 测试规范（第 11 节）
8. 其他约定（第 8/13 节）
```

---

## 16. 前端特殊约定

### 大屏/管理后台隔离
- 管理后台（:3000）和大屏（:3001）是**两个独立 Vite 入口**，共享 `node_modules` 和 `api/client.ts`
- 大屏入口在 `web/admin/screen-src/`，用 `npm run dev:screen` 启动
- 大屏内置独立登录流程，不使用管理后台路由守卫
- 大屏 `screenClient.ts` 为独立 API 客户端（含 Token 过期校验）

### CSS 隔离
| 规则 | 说明 |
|---|---|
| Tailwind preflight 跳过 | `src/index.css` 使用 `@import "tailwindcss/theme"` + `"tailwindcss/utilities"`，跳过 base 层避免与 Ant Design CSS reset 冲突 |
| 深色主题条件激活 | 全局深色样式仅对 `body.xmeco-dark` 生效。大屏入口 `<body class="xmeco-dark">`，管理后台不加 |
| 共享 CSS | 两个入口共享 `src/index.css`，新增组件需在浅色和深色背景下都可用 |

### 前端配置优先级
```
环境变量（VITE_* 前缀）
    ↓
vite.config.ts 中的 define/proxy 配置
    ↓
代码中硬编码默认值（如 API base URL = /api/v1）
```
不存在 `.env` 文件。API 代理通过 Vite proxy 转发到后端 `:9090`。
