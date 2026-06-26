# XMECO AI 协作开发规范 (agents.md)

> 本文档是所有 AI 编码助手在本项目中协作的行为准则和知识基础。
> 请严格遵守下述所有约定。违反核心约束（标注 ⚠️）将导致代码回退。

---

## 1. 项目简介

XMECO（熊猫智控）是一套面向**中央空调水冷系统**的 AIoT 能效管理平台。系统通过 Modbus/TCP 实时采集冷水主机、冷却塔、水泵、电表等设备运行数据，进行时序存储、告警评估、能效分析和智能联动控制，并通过 Web 大屏和小程序进行可视化呈现与远程控制。

**当前开发阶段**：核心骨架已完成（Go 后端 58 条路由、23 张数据库表、11 个测试文件、170 个测试用例全部通过）。前端 15 个页面框架已搭建，但告警管理、遥测历史图表、设备控制面板、启停编排执行监控等页面功能尚待完善。小程序端仅完成 API 客户端和登录流程。WebSocket 实时推送、告警通知渠道、时序数据分区等关键基础设施未实现。

---

## 2. 技术栈

### 后端
| 类别 | 技术 | 版本 |
|------|------|------|
| 语言 | Go | 1.25 |
| HTTP 路由 | `net/http` (标准库 ServeMux) | Go 1.22+ 增强路由 |
| 数据库驱动 | `github.com/jackc/pgx/v5` | v5.10.0 |
| 数据库 | PostgreSQL | 15+ |
| 认证 | `github.com/golang-jwt/jwt/v5` | v5.3.1 |
| 密码 | `golang.org/x/crypto` (bcrypt) | v0.53.0 |
| 日志 | `log/slog` (标准库) | Go 1.21+ |
| 测试 Mock | `github.com/pashagolub/pgxmock/v4` | v4.9.0 |
| 迁移 | `embed` (Go 标准库，嵌入 .sql 文件) | Go 1.16+ |

### Web 管理后台 + 大屏
| 类别 | 技术 | 版本 |
|------|------|------|
| 框架 | React | 19.2.6 |
| UI 库 | Ant Design (`antd`) | 6.4.4 |
| 图标 | `@ant-design/icons` | 6.2.5 |
| 路由 | `react-router-dom` | 7.18.0 |
| HTTP | `axios` | 1.18.0 |
| 时间 | `dayjs` | 1.11.21 |
| 构建 | Vite | 8.0.12 |
| 语言 | TypeScript | ~6.0.2 |
| CSS 框架 | TailwindCSS (v4) | 4.3.1 |
| 代码检查 | ESLint | 10.3.0 |

### 小程序 (UniApp)
| 类别 | 技术 | 版本 |
|------|------|------|
| 框架 | uni-app (Vue 3) | 3.0.0 |
| 语言 | TypeScript | 4.9.4 |
| 构建 | Vite | 5.2.8 |
| 国际化 | vue-i18n | 9.1.9 |

### ⚠️ 技术栈基线锁定

**以上版本号是项目技术栈基线。AI 助手严禁擅自升级任何依赖版本，包括但不限于：**

- `go.mod` 中的 Go 版本和所有 require 模块
- `package.json` 中的任何 dependencies / devDependencies
- Go 编译器版本
- Node.js / npm 版本
- PostgreSQL 版本

**升级唯一允许的时机**：用户**明确**要求升级某个特定依赖，且在对话中给出了"升级到 X.Y.Z"的具体指令。

**原因**：版本变更会引入未知的兼容性问题（API 废弃、行为变更、构建失败），在物联网系统中可能导致硬件通信中断。

**`go.mod` 当前锁定版本**（执行 `go mod tidy` 不得改变以下主版本）：

| 模块 | 锁定版本 | 说明 |
|------|---------|------|
| `github.com/jackc/pgx/v5` | v5.10.0 | 数据库驱动 |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT 认证 |
| `golang.org/x/crypto` | v0.53.0 | bcrypt 密码哈希 |
| `github.com/pashagolub/pgxmock/v4` | v4.9.0 | 数据库 Mock（仅测试） |

**`web/admin/package.json` 锁定的关键依赖**：

| 包名 | 锁定版本 | 不可自动升级 |
|------|---------|-------------|
| `react` / `react-dom` | ^19.2.6 | 主版本 19 |
| `antd` | ^6.4.4 | 主版本 6 |
| `vite` | ^8.0.12 | 主版本 8 |
| `tailwindcss` | 4.3.1 | 主版本 4 |
| `@tailwindcss/vite` | 4.3.1 | Tailwind Vite 插件 |
| `typescript` | ~6.0.2 | 主版本 6 |

**`miniapp/package.json` 锁定的关键依赖**：

| 包名 | 锁定版本 | 不可自动升级 |
|------|---------|-------------|
| `vue` | ^3.4.21 | 主版本 3 |
| `@dcloudio/uni-app` | 3.0.0-40804... | uni-app 版本由 dcloud 控制 |
| `vite` | 5.2.8 | 注意与 Web 端不同 |

---

## 3. 目录结构与模块说明

```
xmeco-new/
├── cmd/server/main.go          # 后端唯一入口：配置加载 → DB 连接 → 迁移 → 服务初始化 → 路由注册 → 启动
├── internal/
│   ├── api/
│   │   ├── handler/            # HTTP Handler 层 — 处理请求参数、调用 Service/Repository、序列化响应
│   │   │   ├── models.go       # 公共工具函数：M (map[string]any)、pathID、queryInt、writeJSON 等
│   │   │   ├── admin.go        # 用户/代理商/角色/权限 CRUD + ResetPassword + SystemInfo
│   │   │   ├── alarm.go        # 告警规则 CRUD + 告警日志查询 + 确认(ack)
│   │   │   ├── auth.go         # 登录(Login) + 当前用户(Me)
│   │   │   ├── dashboard.go    # 大屏配置读写 + ScreenData 一站式聚合端点
│   │   │   ├── intelligence.go # 智能分析代理：能效/预测/策略/电价/电能质量/电表列表
│   │   │   ├── log.go          # 遥测日志/控制日志/统计查询 + CSV 导出
│   │   │   ├── project.go      # 项目 CRUD + 项目用户分配
│   │   │   ├── startup.go      # 启停计划/步骤 CRUD + 执行/监控/停止 + 定时任务 CRUD + 调度运行
│   │   │   ├── telemetry.go    # 实时/历史/统计遥测查询
│   │   │   └── weather.go      # 天气查询代理（城市/省份/当前/项目天气）
│   │   └── middleware/
│   │       ├── auth.go         # JWT 认证中间件（Authorization: Bearer <token>）
│   │       ├── cors.go         # CORS 跨域中间件（可配置域名白名单）
│   │       ├── ratelimit.go    # 登录限流器（令牌桶，默认 10次/分钟/IP）
│   │       ├── permission.go   # RBAC 权限校验中间件（withPerm 包装器）
│   │       └── middleware_test.go
│   ├── config/
│   │   ├── config.go           # 环境变量加载 — 11 个配置项，含默认值和 JWT 强制检查
│   │   └── config_test.go      # 7 个配置测试
│   ├── domain/
│   │   └── models.go           # 领域结构体：Building/Device/DeviceProperty/Register
│   ├── gateway/                # 网关管理 — 硬件通信层
│   │   ├── manager.go          # Manager：双协议监听(Custom + DTU)、设备注册、pollLoop(5并发)
│   │   ├── modbus/
│   │   │   ├── modbus.go       # Modbus RTU 帧构建(BuildReadCommand/BuildWriteCommand)、解析(ParseResponse)、CRC16 校验
│   │   │   └── modbus_test.go  # CRC16 + BuildReadCommand + VerifyCRC 测试
│   │   └── transport/
│   │       ├── types.go        # Transport 接口 (SendAndReceive/Close/IsConnected) + GatewayType 枚举
│   │       ├── custom.go       # 自定义协议：0x68/0x16 帧、注册码 0xE5FF、数据码 0xE4A1、校验和
│   │       └── transparent.go  # DTU 透传：Modbus RTU over TCP、自动 drain(≤10次)、CRC 校验
│   ├── repository/postgres/    # 数据访问层 — 每个 Repo 封装对一张或一组表的 SQL
│   │   ├── models.go           # BuildingRepo/DeviceRepo/PropertyRepo/RegisterRepo — CRUD 方法
│   │   ├── project.go          # ProjectRepo + project_user 关联
│   │   ├── admin.go            # AdminRepo：用户/代理商/角色/权限管理 + 系统信息
│   │   └── *_test.go           # 各 Repo 的 pgxmock 测试
│   └── service/
│       ├── alarm/
│       │   ├── engine.go       # 告警引擎：规则评估(gt/ge/lt/le/eq/range)、部分唯一索引去重、离线告警
│       │   └── engine_test.go  # triggered/parseFloatOK/condCN 测试
│       ├── auth/
│       │   ├── auth.go         # JWT 签发/验证 + bcrypt 哈希 + RBAC 权限查询 + 用户 CRUD
│       │   └── auth_test.go    # 认证服务测试
│       ├── external/weather/
│       │   └── weather.go      # 天气服务：wttr.in(免费 API) → 60min DB 缓存 → 过期回退 + 中英翻译
│       ├── intelligence/       # 多智能体引擎
│       │   ├── analysis.go     # RunFullAnalysis / RunStrategies 聚合入口
│       │   ├── efficiency.go   # 设备能效排行分析
│       │   ├── engine.go       # 湿球温度(Stull 公式) + 冷却塔-主机联动策略
│       │   ├── power_quality.go # 电能质量分析(电压/电流/THD/功率因数/频率 — GB/T 国标)
│       │   ├── predict.go      # 24h 负荷预测(移动平均)
│       │   ├── pump_price.go   # 水泵变频优化 + 分时电价策略
│       │   ├── recommend.go    # 设定点推荐
│       │   └── rotation.go     # 设备轮换策略
│       ├── migration/
│       │   ├── migration.go    # 迁移执行器(Go embed 嵌入 SQL、幂等检测、事务包装)
│       │   └── sql/            # 11 个迁移文件 (001-011)
│       ├── orchestrator/
│       │   ├── orchestrator.go # 启停执行引擎(LoadPlan/StartExecution/Run/Complete)
│       │   └── interlock.go    # 设备联锁检查(条件验证)
│       └── telemetry/
│           └── poller.go       # 设备轮询器：Modbus 读取 → 多格式解码 → pgx Batch 写入 → 告警触发
├── web/admin/                  # Web 管理后台 + 大屏 (React + Ant Design + Vite)
│   ├── package.json            # 依赖和脚本
│   ├── vite.config.ts          # 管理后台 Vite 配置
│   ├── src/                    # 管理后台 (端口 3000)
│   │   ├── App.tsx             # 路由根组件 + 侧边栏布局 (/login → 14 个管理页)
│   │   ├── api/client.ts       # Axios 客户端(JWT 拦截 + 401 自动登出)
│   │   ├── pages/              # 15 个页面组件
│   │   ├── layouts/            # 布局组件(Main.tsx)
│   │   └── components/         # 通用组件(ProtectedRoute 等)
│   └── screen-src/             # 大屏独立入口 (端口 3001)
│       ├── index.html
│       ├── main.tsx
│       ├── vite.config.ts
│       └── ScreenApp.tsx
├── miniapp/                    # uni-app 小程序 (Vue 3)
│   ├── package.json
│   └── src/
│       ├── api/client.ts       # API 客户端(JWT + uni.request + 401 重登)
│       ├── pages/              # 页面组件
│       └── stores/             # Pinia 状态管理
├── tools/                      # 运维工具 (modbus 模拟器、密码重置脚本等)
├── docs/                       # 项目文档
│   └── 系统技术规格与开发路线图.md
├── agents.md                   # 本文档 — AI 协作规范
├── go.mod / go.sum             # Go 模块定义
├── .gitignore                  # Git 忽略规则
├── README.md                   # 项目 README
└── start.bat                   # 一键启动脚本
```

---

## 4. 架构约定

### 4.1 整体架构

**单体分层架构**：项目当前为单体 Go 应用，内部分为严格的四层。

```
HTTP Request
    │
    ▼
┌──────────────────┐
│  Middleware       │  cors → auth(JWT) → rbac(permission)
│  (横向切面)        │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│  Handler          │  参数解析 → 调用 Service/Repository → 序列化 JSON 响应
│  (请求入口)        │  ⚠️ 部分 Handler 直接持有 *pgxpool.Pool 执行 SQL（历史遗留）
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│  Service          │  业务逻辑：告警评估、智能分析、天气缓存、启停编排
│  (业务逻辑)        │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│  Repository       │  SQL 封装：每个 Repo 对应一组表 (Building/Device/Property/Register)
│  (数据访问)        │  ⚠️ Building/Device/Property/Register 四个 Repo CRUD 高度重复
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│  PostgreSQL       │  pgxpool 连接池
└──────────────────┘
```

### 4.2 横向层

- **Gateway Layer**（独立于 HTTP 请求之外）：Manager 启动 TCP 监听，接收网关连接 → 注册设备 → pollLoop 定时轮询 → Modbus 读写 → 遥测写入数据库。
- **Background Tasks**（5 个常驻 goroutine）：设备轮询、告警评估、离线检测(每 1min)、定时启停(每 1min)、数据清理(每 24h)。所有 goroutine 均有 `defer recover()`。

### 4.3 组件间通信

| 通信方式 | 用途 |
|----------|------|
| HTTP REST + JSON | 前端 ↔ 后端（全部 API） |
| `pgxpool.Pool` 直接传递 | Handler/Service/Repository 间共享数据库连接池 |
| Go 接口 (interface) | Gateway → Transport (抽象不同硬件协议) |
| 函数注入 (PollerFn / DeviceLoaderFn) | Manager 通过函数类型注入 poller 和 loader 依赖 |
| 无事件总线 | 当前版本没有消息队列或事件系统 |

### 4.4 数据流走向

```
硬件网关 ──TCP──► Manager ──Transport──► Poller.PollDevice()
                                             │
                                    ┌────────┴────────┐
                                    │  Modbus 读取      │
                                    │  decodeVal 解码   │
                                    │  状态码映射        │
                                    │  Batch 写入 DB    │
                                    │  标记在线          │
                                    │  告警评估          │
                                    └─────────────────┘
                                             │
                                    ┌────────┴────────┐
                                    │  device_telemetry │ ──► 前端查询
                                    │  alarm_log        │ ──► 前端查询
                                    │  device.status    │ ──► 前端查询
                                    └─────────────────┘
```

---

## 5. 编码风格与约定

### 5.1 Go 后端

| 规范项 | 约定 |
|--------|------|
| **缩进** | Tab (`\t`)，与 `gofmt` 一致 |
| **行宽** | 无硬性限制，但单行建议不超过 120 字符 |
| **注释语言** | ⚠️ **所有注释使用中文** |
| **日志** | 统一使用 `slog.Info/Warn/Error`。错误日志必须包含 key=value 上下文（如 `"dev", deviceID, "err", err`） |
| **错误处理** | ⚠️ **严禁使用 `_` 丢弃 error**。出错至少 `slog.Warn()` 记录。<br>仅在 `queryInt()` 等辅助函数中允许丢弃 `Atoi` 错误（语义上 0 即无效值） |
| **空值安全** | 使用 `COALESCE()` 包裹所有 SQL 中可空列；对 `*T` 类型指针取值前必须判 `nil` |
| **结构体 JSON 标签** | 必须同时标注 `json:"xxx"` 字段名（snake_case） |
| **SQL 参数** | 严格使用 `$1, $2, ...` 占位符，**禁止字符串拼接用户输入**。动态参数（如 `interval`）必须白名单校验 |
| **Repository GetByID** | ⚠️ 查询不到记录时返回 `nil, nil`（不是 error）。用 `errors.Is(err, pgx.ErrNoRows)` 区分 "未找到" 和 "数据库异常"。Handler 先判 `err != nil`（→500），再判 `obj == nil`（→404） |
| **DBTX 接口** | Repository 构造函数接受 `DBTX` 接口（含 `*pgxpool.Pool` 和 `pgx.Tx`），而非具体类型。这样 SetProjectUsers 等方法可以在事务中复用 Repo 方法 |

#### 5.1.1 命名规范细则

Go 后端的所有命名遵循 **"用什么语言写代码，就用什么语言命名"** 原则 —— 本项目代码语言为英文，因此所有标识符使用英文。

| 类别 | 规则 | 正确示例 | 错误示例 |
|------|------|---------|---------|
| **包 (package)** | 小写、单数、无分隔符 | `handler` `telemetry` `middleware` | `handlers` `my_handler` |
| **Go 文件名** | 小写 + 下划线分隔 | `rate_limit.go` `engine_test.go` | `RateLimit.go` `engine-test.go` |
| **接口 (interface)** | 动词或 -er 结尾 | `Transport` `DeviceLoaderFn` | `ITransport` |
| **结构体 (struct)** | PascalCase (大驼峰) | `BuildingHandler` `AlarmEngine` | `building_handler` |
| **结构体字段** | PascalCase（导出）/ camelCase（私有）+ JSON 标签 snake_case | `DeviceID int \`json:"device_id"\`` | `device_id int` |
| **函数/方法** | PascalCase（导出）/ camelCase（私有） | `GetByID()` `decodeVal()` `pathID()` | `get_by_id()` |
| **变量** | camelCase，简短有意义 | `deviceID` `gwMgr` `argIdx` | `did` `gm` `ai` |
| **常量** | PascalCase | `frameStart` `defaultJWTSecret` | `FRAME_START` `DEFAULT_JWT_SECRET` |
| **SQL 迁移文件名** | `NNN_英文描述.sql`，NNN 为三位递增序号 | `012_add_device_snapshot.sql` | `12-添加设备快照.sql` |
| **测试文件名** | 源文件名 + `_test.go` | `auth_test.go` `modbus_test.go` | `test_auth.go` |

#### 5.1.2 常见命名对照表

| 中文概念 | Go 标识符 | 说明 |
|----------|-----------|------|
| 项目 | `Project` | Domain struct |
| 楼宇 | `Building` | Domain struct |
| 设备 | `Device` | Domain struct |
| 属性 | `DeviceProperty` / `Property` | 文件用简写 |
| 寄存器 | `Register` | Domain struct |
| 告警 | `Alarm` | 规则 `AlarmRule`，日志 `AlarmLog` |
| 启停 | `Startup` | 计划 `StartupPlan`，步骤 `StartupStep` |
| 编排 | `Orchestrator` | 包名和类型名 |
| 遥测 | `Telemetry` | Handler/Service 类型名 |
| 网关 | `Gateway` | 管理器 `Manager`，传输层 `Transport` |
| 轮询器 | `Poller` | 类型名 |
| 代理商 | `Agent` | Domain struct |
| 权限 | `Permission` | Domain struct |
| 电能质量 | `PowerQuality` | 分析结果类型名 |

### 5.2 TypeScript / React 前端

| 规范项 | 约定 |
|--------|------|
| **缩进** | 2 空格 |
| **引号** | 单引号 `'` |
| **分号** | 省略 |
| **类型** | 强制 TypeScript 严格模式 (`tsc -b` 编译检查) |
| **文件命名** | 组件文件：`PascalCase.tsx` (如 `Login.tsx`)；工具文件：`camelCase.ts` (如 `client.ts`) |
| **组件** | 函数式组件 + Hooks，不使用 Class Component |
| **API 调用** | 统一使用 `src/api/client.ts` 中导出的 `api` 实例 |
| **状态管理** | React 内置 `useState/useEffect`，管理后台无全局状态库 |
| **UI 组件** | 强制使用 `antd` 组件（Button/Table/Form/Modal 等） |

### 5.3 小程序 (Vue 3)

| 规范项 | 约定 |
|--------|------|
| **缩进** | 2 空格 |
| **API** | 组合式 API (`<script setup lang="ts">`) |
| **状态管理** | Pinia (`src/stores/`) |
| **组件命名** | PascalCase |
| **Uni API** | 使用 `uni.request` 等 uni-app 特有 API |

---

## 6. 常用命令

### 6.1 Go 后端

```bash
# 构建
cd d:\py\xmeco-new
go build ./...

# 运行
go run ./cmd/server/main.go
# 或使用预编译的 server.exe（通过 start.bat 构建）

# 运行所有测试
go test ./internal/...

# 运行特定包测试（含详细输出）
go test -v ./internal/api/handler/
go test -v ./internal/config/
go test -v ./internal/gateway/modbus/
go test -v ./internal/service/alarm/
go test -v ./internal/service/auth/
go test -v ./internal/repository/postgres/

# 静态检查
go vet ./...
```

### 6.2 Web 管理后台

```bash
cd d:\py\xmeco-new\web\admin

# 安装依赖（首次）
npm install

# 启动管理后台（端口 3000）
npm run dev

# 启动大屏（端口 3001）
npm run dev:screen

# 编译检查
npm run build

# 代码检查
npm run lint
```

### 6.3 小程序

```bash
cd d:\py\xmeco-new\miniapp

# 安装依赖
npm install

# 启动 H5 开发服务器（端口 5173）
npm run dev:h5

# 启动微信小程序
npm run dev:mp-weixin

# 类型检查
npm run type-check
```

---

## 7. 环境变量与配置

### 7.1 后端配置（环境变量）

配置文件位置：`internal/config/config.go` — 在 `Load()` 函数中集中读取。

| 变量名 | 默认值 | 用途 | 备注 |
|--------|--------|------|------|
| `XMECO_DB_HOST` | `localhost` | PostgreSQL 主机 | |
| `XMECO_DB_PORT` | `5432` | PostgreSQL 端口 | |
| `XMECO_DB_USER` | `postgres` | 数据库用户名 | |
| `XMECO_DB_PASSWORD` | `xmeco123` | 数据库密码 | 生产环境务必修改 |
| `XMECO_DB_NAME` | `xmeco` | 数据库名称 | |
| `XMECO_SERVER_PORT` | `9090` | HTTP 监听端口 | |
| `XMECO_JWT_SECRET` | ⚠️ 无默认值 | JWT 签名密钥 (HS256) | **生产环境缺省值将拒绝启动 (`os.Exit(1)`)** |
| `XMECO_ALLOWED_ORIGINS` | `*` | CORS 允许的来源（逗号分隔域名） | 生产环境建议设为具体域名，如 `https://your-domain.com`。<br>⚠️ 使用 `*` 时若请求携带 `Authorization` 头，CORS 中间件会回显 `Origin` 而非 `*`（Fetch 规范禁止 `*` + 凭证） |
| `XMECO_RETENTION_DAYS` | `730` | 遥测数据保留天数 | `0` = 不清理 |
| `XMECO_POLL_INTERVAL_SEC` | `3` | 设备轮询间隔（秒） | 过小可能导致单网关设备 poll 超时 |

**⚠️ 安全规则**：日志中禁止输出含密码的 DSN。使用 `config.MaskedDSN()` 方法：
```go
slog.Info("db connected", "dsn", cfg.MaskedDSN())   // postgres://user:***@host:5432/db?sslmode=disable
db, err := postgres.New(ctx, cfg.DSN())               // 真实连接使用完整 DSN
```

### 7.2 前端配置

- Web 管理后台：`web/admin/vite.config.ts`
- 大屏：`web/admin/screen-src/vite.config.ts`
- 小程序：`miniapp/vite.config.ts`
- API base URL：前端代码中硬编码 `/api/v1`，通过 Vite proxy 转发到后端

---

## 8. 已完成与待完成清单

### 8.1 已完成（✅）

| 模块 | 内容 | 文件 |
|------|------|------|
| **数据库** | 23 张表全部建表 + 11 个版本迁移自动执行 | `internal/service/migration/sql/` |
| **认证** | JWT 签发/验证 + bcrypt 密码 + RBAC 37 权限码 | `internal/service/auth/` |
| **路由** | 58 条全部 API 已注册 | `cmd/server/main.go` |
| **项目管理** | CRUD + 用户分配 + 城市级联 | `internal/api/handler/project.go` |
| **楼宇/设备/属性/寄存器** | 完整 CRUD 四件套（含 Repository 层） | `internal/api/handler/models.go` |
| **告警引擎** | 规则评估(6种条件) + 原子去重 + 离线告警 | `internal/service/alarm/engine.go` |
| **启停编排** | 计划/步骤 CRUD + 执行(goroutine) + 步骤日志 + 联锁检查 | `internal/api/handler/startup.go` |
| **定时任务** | once/daily/weekly 调度 + 硬件控制 | `internal/api/handler/startup.go` |
| **遥测查询** | 实时/历史/统计 + CSV 导出 + 聚合间隔 | `internal/api/handler/telemetry.go` / `log.go` |
| **大屏端点** | ScreenData 一站式聚合（项目/建筑/设备/天气/任务/告警/能耗/电表） | `internal/api/handler/dashboard.go` |
| **天气服务** | wttr.in 免费 API + 60min DB 缓存 + 过期回退 + 中英翻译 | `internal/service/external/weather/` |
| **智能分析** | 能效分析、负荷预测、设定点推荐、联动策略、水泵优化、分时电价、电能质量、设备轮换 | `internal/service/intelligence/` |
| **网关协议** | Modbus RTU + 自定义协议(0x68/0x16) + DTU 透传 | `internal/gateway/` |
| **设备轮询** | 3s 周期 Modbus 批量采集 + 多格式解码 + pgx Batch 写入 | `internal/service/telemetry/poller.go` |
| **后台守护** | 数据保留(每24h分片) + 离线检测(每1min) + goroutine panic recovery | `cmd/server/main.go` |
| **安全修复** | CORS 凭证安全检测、JWT 强制、密码重置强制旧密码、SQL 注入防护、空指针防护、DSN 密码脱敏日志 | 多个文件 |
| **权限常量** | `auth.Perm` 结构体集中管理 36 个权限码常量，路由注册零裸字符串 | `internal/service/auth/rbac.go` |
| **代码审查** | 12+ 项严重/高优先级缺陷修复：JSON 序列化、优雅关闭、GetByID 错误区分、中间件 Content-Type 等 | 10+ 文件 |
| **路由注册重构** | 提取到 `registerRoutes()` 函数，使用权限常量，main() 从 349 行缩减至 ~180 行 | `cmd/server/main.go` |
| **Web 管理后台** | 15 个页面框架已搭建，登录 + 项目/楼宇/设备/属性/寄存器 CRUD 可用 | `web/admin/src/pages/` |
| **Web 大屏** | 深色全屏 + 设备拓扑 + 实时数据卡片 + 天气 + 告警 + 能耗 | `web/admin/src/pages/Screen.tsx` |
| **小程序** | API client + 登录流程 | `miniapp/src/` |
| **测试** | 11 个测试文件，~170 个测试用例，全部通过 | 各 `*_test.go` 文件 |

### 8.2 待完成（❌）

| 优先级 | 任务 | 位置 |
|--------|------|------|
| **P0** | Web 端：告警管理页面功能完善（规则 CRUD 表单 + 日志列表 + ack 操作） | `web/admin/src/pages/Alarms.tsx` |
| **P0** | Web 端：遥测历史图表（Recharts 时序折线图 + 指标筛选 + 时间范围） | `web/admin/src/pages/Telemetry.tsx` |
| **P0** | Web 端：设备控制面板（开关机 Switch + 设值 InputNumber） | `web/admin/src/pages/Devices.tsx` |
| **P0** | Web 端：启停编排执行监控（实时步骤进度条 + 执行日志） | `web/admin/src/pages/StartupPlans.tsx` |
| **P0** | 小程序端：登录 + 设备列表 + 实时数据卡片 + 告警列表 | `miniapp/src/pages/` |
| **P1** | WebSocket 实时推送 — 替换前端 30s 轮询 | `internal/api/handler/dashboard.go` + 新建 WS handler |
| **P1** | 告警通知渠道 — 短信/公众号/邮件 | 新建 `internal/service/notify/` |
| **P1** | 时序数据分区 — TimescaleDB hypertable 或 date partition | 新建迁移文件 |
| **P1** | 设备控制闭环 — Modbus 写后读回确认 | `internal/api/handler/models.go` dispatchHardware |
| **P2** | 能效分析报告 — 日报/周报/月报 + PDF 导出 | 新建 `internal/service/report/` |
| **P2** | 多级权限数据隔离 — Agent 级项目/楼宇/设备可见性 | `internal/api/middleware/permission.go` + Repository 层 |
| **P2** | 设备模板批量导入 — Excel 模板 + 事务批量写入 | 新建 `internal/api/handler/import.go` |
| **P2** | Prometheus 指标 — `/metrics` 端点 | `cmd/server/main.go` |
| **P3** | Repository 泛型重构 — 消除 CRUD 重复代码 | `internal/repository/postgres/models.go` |
| **P3** | Handler 中 SQL 下沉到 Service — Dashboard/Log/Startup/Alarm Handler | `internal/api/handler/` 相关文件 |
| **P3** | 核心模块测试补全 — gateway 协议解析 + poller 解码 + alarm 引擎 | 各模块 |

---

## 9. AI 协作指南

### 9.1 只读/自动生成文件（不要修改）

| 文件/目录 | 原因 |
|-----------|------|
| `go.sum` | 由 `go mod tidy` 自动生成 |
| `web/admin/dist/` | 构建产物 |
| `.gitignore` 中忽略的文件 | 不需要修改，除非新增了需要提交/忽略的文件类型 |

### 9.2 修改时需格外小心的模块

| 模块 | 风险 | 注意事项 |
|------|------|---------|
| `internal/gateway/transport/custom.go` | 自定义协议帧解析，任何字节偏移错误都会导致网关通信中断 | 修改后须与模拟器联调验证；`frame[7]`/`frame[8]` 分别对应 `customCmdHi`/`customCmdLo` 常量 |
| `internal/gateway/transport/transparent.go` | DTU 透传 Modbus，Drain 循环有上限保护(10次) | 修改 `SendAndReceive` 时注意互斥锁 `t.mu.Lock()` 的正确释放 |
| `internal/service/telemetry/poller.go` | 核心数据采集链路，`decodeVal` 涉及多字节序/掩码/倍率 | 修改解码逻辑后须验证所有 `DataOrder`（高位在前/低位在前/低字在前） |
| `internal/service/alarm/engine.go` | 告警去重依赖部分唯一索引 `(device_id, alarm_type) WHERE ack_at IS NULL` | 修改 `INSERT ... ON CONFLICT` 语句时注意索引条件 |
| `internal/config/config.go` | JWT 密钥缺省值触发 `os.Exit(1)` | 不要在 `Load()` 中移除该检查；测试中必须 `os.Setenv("XMECO_JWT_SECRET", "test-secret")` |
| `cmd/server/main.go` | 路由注册 + 5 个 goroutine 启动 | 新增路由须同时注册权限码；修改 goroutine 必须确保 `defer recover()` |

### 9.3 添加新功能的标准步骤

遵循现有的分层约定，按以下顺序实现：

**新增一个 CRUD 实体（如 Sensor）：**

```
1. domain/models.go           → 添加 Sensor struct
2. repository/postgres/       → 新建 sensor.go，实现 SensorRepo{pool}.List/GetByID/Create/Update/Delete
3. api/handler/               → 新建 sensor.go，实现 SensorHandler，每个方法遵循现有模式：
                                 - 参数解析 (queryInt/pathID/json.Decode)
                                 - 调用 repo 方法
                                 - ok(w, data) / created(w, data) / notFound(w, msg)
4. cmd/server/main.go         → 在 main() 中实例化 repo 和 handler；
                                 → 在 registerRoutes() 中注册路由 + withPerm(authSvc, auth.Perm.XxxCode, h.Method)
5. migration/sql/             → 新建 012_xxx.sql，包含 CREATE TABLE IF NOT EXISTS
```

**新增一个 Service 逻辑（如通知服务）：**

```
1. service/notify/            → 新建目录，实现 Service struct + New() 构造函数
2. 构造函数接受 *pgxpool.Pool 和必要的配置
3. 在 cmd/server/main.go 中实例化并注入到需要的 Handler
4. Handler 通过构造函数接受该 Service
```

### 9.4 代码模式速查

**Handler 标准模式（带 Repository）：**
```go
type FooHandler struct{ repo *postgres.FooRepo }
func NewFooHandler(r *postgres.FooRepo) *FooHandler { return &FooHandler{r} }

func (h *FooHandler) List(w http.ResponseWriter, r *http.Request) {
    list, err := h.repo.List(r.Context(), queryInt(r, "parent_id"))
    if err != nil { serverErr(w, err); return }
    if list == nil { list = []domain.Foo{} }
    ok(w, list)
}
```

**Handler 标准模式（直接 SQL，仅限已有模式的模块）：**
```go
type BarHandler struct{ pool *pgxpool.Pool }
func NewBarHandler(pool *pgxpool.Pool) *BarHandler { return &BarHandler{pool} }

func (h *BarHandler) Get(w http.ResponseWriter, r *http.Request) {
    rows, err := h.pool.Query(r.Context(), "SELECT ...", args...)
    if err != nil { serverErr(w, err); return }
    defer rows.Close()
    // scan + build response...
}
```

**Repository 标准模式（含 GetByID nil/nil 约定）：**
```go
type FooRepo struct{ pool DBTX }  // DBTX 接口，接受 *pgxpool.Pool 或 pgx.Tx
func NewFooRepo(pool DBTX) *FooRepo { return &FooRepo{pool} }

func (r *FooRepo) List(ctx context.Context, parentID int) ([]domain.Foo, error) {
    rows, err := r.pool.Query(ctx, "SELECT ... WHERE ($1=0 OR parent_id=$1)", parentID)
    if err != nil { return nil, err }
    defer rows.Close()
    var list []domain.Foo
    for rows.Next() {
        var f domain.Foo
        if err := rows.Scan(&f.ID, ...); err != nil { return nil, err }
        list = append(list, f)
    }
    return list, rows.Err()
}

func (r *FooRepo) GetByID(ctx context.Context, id int) (*domain.Foo, error) {
    var f domain.Foo
    err := r.pool.QueryRow(ctx, "SELECT ... WHERE id=$1", id).Scan(&f.ID, ...)
    if errors.Is(err, pgx.ErrNoRows) { return nil, nil }  // 未找到：返回 nil, nil
    return &f, err                                         // 错误或成功
}
```

**⚠️ GetByID 约定**：查询不到记录时返回 `(nil, nil)`（不是 error）。Handler 必须先判 `err != nil` → 500，再判 `obj == nil` → 404。

**Handler Get 标准模式（含 nil 检查）：**
```go
func (h *FooHandler) Get(w http.ResponseWriter, r *http.Request) {
    obj, err := h.repo.GetByID(r.Context(), pathID(r))
    if err != nil { serverErr(w, err); return }
    if obj == nil { notFound(w, "资源不存在"); return }
    ok(w, obj)
}
```

**路由注册 + 权限常量模式：**
```go
// 路由注册集中在 registerRoutes() 函数中（cmd/server/main.go）
// 权限码使用 auth.Perm.* 常量，禁止裸字符串
func registerRoutes(db *postgres.DB, rl *middleware.RateLimiter, authSvc *auth.Service, ...) *http.ServeMux {
    p := auth.Perm  // 简写
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

**⚠️ 重要：Handler 与 Repository 不可混用。如果创建了 Repo，Handler 必须通过 Repo 访问数据库，而不是直接持有 `*pgxpool.Pool`。当前 DashboardHandler/LogHandler/StartupHandler 直接 SQL 是历史遗留，新功能不要效仿。**

---

## 10. 其他约定

### 10.1 Git 工作流

- **分支**：当前仅 `master` 分支。建议新功能在功能分支开发，合并前通过 `go test ./...` 和 `go vet ./...`。
- **提交消息**：使用中文，格式建议：`类型: 简要描述`（如 `fix: 修复密码重置免旧密码漏洞`、`feat: 新增告警短信通知渠道`）。
- **不要提交**：`server.exe`、`node_modules/`、`dist/`、`.env`。

### 10.2 数据库迁移规范

#### 10.2.1 迁移文件命名

迁移文件位于 `internal/service/migration/sql/`，命名格式：

```
NNN_英文下划线描述.sql
```

| 要素 | 规则 | 示例 |
|------|------|------|
| 序号 | 三位数字，递增（`001_`, `002_`, ...） | `009_`, `010_`, `011_` |
| 描述 | 英文，下划线分隔，简明描述变更内容 | `project_user_table` `add_last_online_at` |

**正确示例**：
- `009_project_user_table.sql`
- `010_add_device_no_index.sql`
- `011_add_scheduled_task_enabled_index.sql`

**错误示例**：
- `9_project_user.sql`（序号不足三位）
- `010-添加定时任务.sql`（用中文、用连接符）
- `010.sql`（无描述）

#### 10.2.2 迁移执行机制

- 迁移通过 Go 的 `embed` 将 SQL 文件嵌入编译后的二进制，启动时自动执行
- `internal/service/migration/migration.go` 负责执行逻辑：
  - 读取 `schema_migrations` 表，获取已应用的版本号
  - 按文件名字典序排序，逐个对比
  - 跳过已应用的版本，仅执行新的
  - 每个迁移文件在单独的事务中执行
- **所有迁移必须幂等**：使用 `IF NOT EXISTS`、`ON CONFLICT DO NOTHING`、`ADD COLUMN IF NOT EXISTS` 等

#### 10.2.3 如何新增迁移

```bash
# 1. 确定最新序号（查看 sql/ 目录中最大数字）
ls internal/service/migration/sql/

# 2. 创建新文件（假设当前最大为 011）
# 文件命名：internal/service/migration/sql/012_your_description.sql

# 3. 文件内容示例：
# -- 012_your_description.sql
# CREATE TABLE IF NOT EXISTS your_table (
#     id SERIAL PRIMARY KEY,
#     ...
# );

# 4. 无需修改 migration.go — embed 会自动包含新的 .sql 文件

# 5. 重启后端，迁移自动执行
```

#### 10.2.4 迁移约束

| ⚠️ 规则 | 说明 |
|----------|------|
| **严禁修改已有迁移** | 已合入主分支的迁移文件不可修改。任何 Schema 变更必须通过新迁移完成 |
| **严禁删除迁移** | 删除迁移文件会导致 `schema_migrations` 与实际 Schema 不一致 |
| **迁移不可回滚** | 当前无 down 迁移机制，变更前务必在测试环境验证 |
| **禁用 DROP** | 迁移中不要使用 `DROP TABLE` / `DROP COLUMN`。如需删除列，先在代码中停止引用，单独一个迁移处理 |

### 10.3 缓存策略

#### 10.3.1 当前缓存现状

项目**没有引入 Redis 或内存缓存库**。当前唯一缓存是天气服务使用的 **数据库表缓存**：

| 缓存对象 | 存储位置 | TTL | 淘汰策略 | 代码位置 |
|----------|---------|-----|---------|----------|
| 天气数据 | `weather_cache` 表 | 60 分钟 | 查询时检查 `expires_at > NOW()` | `internal/service/external/weather/weather.go:67-70` |

该缓存还支持"过期回退"：当 wttr.in API 调用失败时，会查询 `weather_cache` 中最新一条记录（不限过期时间），作为降级数据返回（同文件第 166-179 行）。

#### 10.3.2 何时引入缓存

判断标准：**同一数据在短时间内被多次查询，且数据源响应慢或成本高，才引入缓存。**

| 场景 | 建议方案 |
|------|---------|
| **读多写少、允许一定延迟** | 数据库表缓存（仿照 `weather_cache` 模式）。建 `_cache` 后缀表，写入时写 `expires_at`，查询时先查缓存 |
| **大屏 `/screen/data`** | 引入 `sync.Map` 或 `groupcache` 内存缓存，TTL 3-5 秒，每次 poll 写入后刷新 |
| **告警去重** | 当前已用 PostgreSQL 部分唯一索引 `(device_id, alarm_type) WHERE ack_at IS NULL`，无需额外缓存 |
| **用户 session/登录态** | 当前无需额外缓存，JWT 无状态验证 |

#### 10.3.3 缓存实现规范

**数据库表缓存（推荐用于低频数据）**：

```sql
-- 迁移中建表
CREATE TABLE IF NOT EXISTS foo_cache (
    cache_key   VARCHAR(200) PRIMARY KEY,
    payload     JSONB NOT NULL,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_foo_cache_expires ON foo_cache(expires_at);
```

```go
// Service 中查询
func (s *Service) getCached(ctx context.Context, key string) (*Data, bool) {
    var d Data
    err := s.pool.QueryRow(ctx,
        `SELECT payload FROM foo_cache WHERE cache_key=$1 AND expires_at > NOW()`, key).
        Scan(&d)
    return &d, err == nil
}
```

**内存缓存（推荐用于高频数据）**：

```go
// 仅在确有必要时使用 sync.Map，避免过度使用
var screenCache sync.Map

type cacheEntry struct {
    data      map[string]any
    expiresAt time.Time
}

func getCached(key string) (map[string]any, bool) {
    v, ok := screenCache.Load(key)
    if !ok { return nil, false }
    entry := v.(*cacheEntry)
    if time.Now().After(entry.expiresAt) { return nil, false }
    return entry.data, true
}
```

#### 10.3.4 缓存禁用场景

以下情况**不要引入缓存**：
- 实时遥测数据（`device_telemetry`）——数据由 poller 高频写入，缓存会导致数据滞后
- 设备控制操作——必须实时下发，不可缓存
- 告警日志查询——用户期望看到最新告警
- 任何需要强一致性的写入路径

### 10.4 测试规范

#### 10.4.1 硬性要求

**⚠️ 新增代码必须附带测试。具体要求按模块分级：**

| 模块类型 | 测试要求 | 测试方式 | 理由 |
|----------|---------|---------|------|
| **Repository**（新建 Repo 或新增方法） | 🔴 必须 | `pgxmock` 表驱动测试 | 数据访问是系统基石，SQL 错误会污染数据库 |
| **Gateway / Transport**（协议解析/帧构建） | 🔴 必须 | 纯单元测试（字节数组 in/out） | 协议解析错误会导致硬件通信中断，极难线上排查 |
| **Service**（告警引擎/编排器/智能分析） | 🟡 必须（含业务逻辑的方法） | `pgxmock` + 表驱动测试 | 业务逻辑错误产生误报/漏报，直接影响能效决策 |
| **Handler**（新建 Handler 或新增端点） | 🟡 必须（至少覆盖正常路径和错误路径） | `httptest.NewRecorder` + mock Service | 确保 API 契约正确，输入校验有效 |
| **工具函数**（decodeVal / parseStatusMapping 等） | 🟢 强烈建议 | 表驱动测试 | 工具函数被多处调用，一次遗漏影响面广 |
| **前端页面** | 🟢 暂不强制 | — | 前端仍在快速迭代中，页面结构不稳定 |

**"必须"的含义**：缺少对应测试的 PR 不得合入。CI 中 `go test ./...` 零失败是对应的最低门禁。

#### 10.4.2 测试文件与测试函数命名

| 规则 | 说明 |
|------|------|
| 测试文件命名 | `xxx_test.go`，与源文件同目录、同包 |
| 测试函数命名 | `TestXxx(t *testing.T)` — 例如 `TestDecodeValFloat`、`TestTriggeredRange` |
| 表驱动测试 | 优先使用 `[]struct{name string, input ..., want ...}` 模式，每个 case 用 `t.Run(tt.name, ...)` 运行 |
| 环境变量依赖 | 测试调用 `config.Load()` 前须 `os.Setenv("XMECO_JWT_SECRET", "test-secret")` |
| 独立性 | 所有测试必须能独立运行，不依赖外部服务（DB/Redis/网络） |

#### 10.4.3 各层测试模板

**Repository 测试（pgxmock）**：

```go
func TestFooRepo_List(t *testing.T) {
    mock, err := pgxmock.NewPool()
    if err != nil { t.Fatal(err) }
    defer mock.Close()

    rows := mock.NewRows([]string{"id", "name"}).
        AddRow(1, "test")
    mock.ExpectQuery(`SELECT id, name FROM foo`).WillReturnRows(rows)

    repo := NewFooRepo(mock)
    list, err := repo.List(context.Background(), 0)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if len(list) != 1 { t.Errorf("want 1, got %d", len(list)) }

    if err := mock.ExpectationsWereMet(); err != nil { t.Errorf("unmet: %v", err) }
}

func TestFooRepo_Create(t *testing.T) {
    mock, _ := pgxmock.NewPool()
    defer mock.Close()

    mock.ExpectQuery(`INSERT INTO foo`).WithArgs("bar").
        WillReturnRows(mock.NewRows([]string{"id"}).AddRow(1))

    repo := NewFooRepo(mock)
    id, err := repo.Create(context.Background(), "bar")
    if err != nil { t.Fatalf("unexpected: %v", err) }
    if id != 1 { t.Errorf("want 1, got %d", id) }
}
```

**Gateway 协议测试（纯字节）**：

```go
func TestCustomTransportUnwrap_Valid(t *testing.T) {
    // 构造一个合法的自定义协议帧
    mac := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
    payload := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01} // Modbus 读保持寄存器
    tport := NewCustomTransport(nil, mac) // conn=nil 只测帧逻辑
    frame := tport.wrap(payload)

    result, err := tport.unwrap(frame)
    if err != nil { t.Fatalf("unwrap failed: %v", err) }
    if !bytes.Equal(result, payload) { t.Errorf("payload mismatch") }
}

func TestCustomTransportUnwrap_ShortFrame(t *testing.T) {
    tport := NewCustomTransport(nil, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06})
    _, err := tport.unwrap([]byte{0x68, 0x01}) // 太短
    if err == nil { t.Error("expected error for short frame") }
}

func TestCustomTransportUnwrap_BadChecksum(t *testing.T) {
    tport := NewCustomTransport(nil, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06})
    // 构造一个校验和错误的帧
    frame := tport.wrap([]byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01})
    frame[len(frame)-3] ^= 0xFF // 破坏校验和
    _, err := tport.unwrap(frame)
    if err == nil { t.Error("expected checksum error") }
}
```

**Service 表驱动测试（已有模式参考）**：

```go
func TestTriggered(t *testing.T) {
    tests := []struct {
        name      string
        cond      string
        val, thr  float64
        minV, maxV string
        want      bool
    }{
        {"gt_true",  "gt", 25.0, 20.0, "", "", true},
        {"gt_false", "gt", 15.0, 20.0, "", "", false},
        {"lt_true",  "lt", 15.0, 20.0, "", "", true},
        {"range_out", "range", 5.0, 0, "10", "30", true},
        {"range_in",  "range", 20.0, 0, "10", "30", false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := triggered(tt.cond, tt.val, tt.thr, tt.minV, tt.maxV)
            if got != tt.want { t.Errorf("triggered() = %v, want %v", got, tt.want) }
        })
    }
}
```

**Handler 测试（httptest）**：

```go
func TestFooHandler_Get(t *testing.T) {
    mock, _ := pgxmock.NewPool()
    defer mock.Close()

    rows := mock.NewRows([]string{"id", "name"}).AddRow(1, "test")
    mock.ExpectQuery(`SELECT id, name FROM foo WHERE id=\$1`).
        WithArgs(1).WillReturnRows(rows)

    h := NewFooHandler(NewFooRepo(mock))
    req := httptest.NewRequest("GET", "/api/v1/foo/1", nil)
    rec := httptest.NewRecorder()
    h.Get(rec, req)

    if rec.Code != http.StatusOK { t.Errorf("status = %d, want 200", rec.Code) }
}
```

#### 10.4.4 不需要测试的情况

以下情况**可以不写测试**，避免过度工程化：

| 情况 | 理由 |
|------|------|
| `cmd/server/main.go` 中的路由注册和 goroutine 启动 | 集成/启动逻辑，单元测试价值低 |
| 纯数据定义文件（`domain/models.go` 中的 struct） | 无逻辑可测 |
| SQL 迁移文件（`.sql`） | 依赖真实数据库，应由迁移执行器集成测试覆盖 |
| 第三方库的简单包装（如 `itos`/`ftoa`） | 不值得测试 |
| 前端纯展示组件（无交互逻辑） | 视觉验证即可 |

#### 10.4.5 测试检查清单（新增代码用）

```
新增 Repository 方法：
□ 正常路径：模拟 QueryRow/Query 返回正确数据 → 断言返回值
□ 空结果路径：模拟返回 pgx.ErrNoRows → 断言返回 nil/空切片 和 nil error
□ 错误路径：模拟返回 fmt.Errorf → 断言 error 非 nil

新增 Gateway 协议解析：
□ 正常帧：构造合法帧 → 断言解析正确
□ 短帧：帧长度不足 → 断言返回 error
□ 校验错：破坏校验和/CRC → 断言返回 error
□ 边界值：最小长度帧、最大长度帧

新增 Service 业务逻辑：
□ 正常输入 → 断言输出符合预期
□ 边界输入（0、负数、极大值、空字符串）→ 断言不 panic，返回合理值
□ 至少 3 个表驱动 case

新增 Handler 端点：
□ 正常请求 → 断言 HTTP 200 和 JSON body 结构
□ 参数缺失/无效 → 断言 HTTP 400
□ 资源不存在 → 断言 HTTP 404
```

### 10.5 接口安全性

- 所有 `/api/v1/*` 路由（除 `/auth/login` 和 `/health`）必须经过 JWT 认证。
- 非公开路由必须通过 `withPerm(authSvc, "perm.code", handler)` 包装。
- 登录接口受 `rateLimiter.LimitLogin()` 限流保护。

### 10.6 前端与大屏隔离

- 管理后台（端口 3000）和大屏（端口 3001）是**两个独立的 Vite 入口**，共享 `node_modules` 和 `api/client.ts`。
- 大屏入口文件在 `web/admin/screen-src/`，使用 `npm run dev:screen` 启动。
- 大屏内置独立的登录流程（Screen.tsx 内含 Login 状态切换），不使用管理后台的路由守卫。
- 新增页面文件放在 `web/admin/src/pages/`，并在 `App.tsx` 中注册路由。

#### 10.6.1 CSS 隔离约束

| 规则 | 说明 |
|------|------|
| **Tailwind preflight 跳过** | `src/index.css` 使用 `@import "tailwindcss/theme"` + `@import "tailwindcss/utilities"`，**跳过 base/preflight 层**以避免与 Ant Design 的 CSS reset 冲突。开发者不应依赖 Tailwind 的 base 行为（如 `border` 默认值、`ring` 默认宽度等），需显式指定 |
| **深色主题条件激活** | 全局深色样式仅对 `body.xmeco-dark` 生效。大屏入口 `screen-src/index.html` 的 `<body>` 添加 `class="xmeco-dark"`，管理后台入口不添加 |
| **共享 CSS 文件** | 两个入口共享同一份 `src/index.css`。新增组件样式时需确保在浅色（管理后台）和深色（大屏）背景下都可用 |
| **JWT 工具函数共享** | `src/utils/auth.ts` 中的 `isTokenExpired()` 被 `ProtectedRoute` 和 `Screen.tsx` 共用，新增入口时也应复用，不要重复实现 |

### 10.7 配置优先级

### 10.7 配置优先级

```
环境变量 (最高优先级)
    ↓
config.Load() 中的默认值 (fallback)
```

不存在配置文件。所有配置通过环境变量注入。生产部署时通过 Docker Compose 或 systemd 的 `Environment=` 传递环境变量。

---

## 11. 硬性规则与禁止事项

以下规则**无条件强制执行**，违反即属于代码缺陷。

### 11.1 依赖方向（单向不可逆）

```
┌──────────────────────────────────────────────────┐
│                  依赖方向：自上而下                  │
│                                                    │
│   Handler ──────► Service ──────► Repository       │
│    (绝不反向)      (绝不反向)       │                │
│                         ▲          ▼                │
│                         │    ┌──────────┐          │
│                         │    │ PostgreSQL│          │
│                         │    └──────────┘          │
│                         │                          │
│              Domain (models.go)                    │
│               ← 被所有层引用，不引用任何层           │
└──────────────────────────────────────────────────┘
```

| 规则 | 说明 |
|------|------|
| Handler → Service | ✅ 允许。Handler 调用 Service 方法，Service 通过构造函数注入 |
| Handler → Repository | ⚠️ 当前**暂时允许**（历史遗留），新功能**禁止** |
| Handler → `*pgxpool.Pool` | ❌ 新功能禁止。必须通过 Repository 或 Service 访问数据库 |
| Service → Repository | ✅ 允许。Service 调用 Repository 执行数据访问 |
| Service → `*pgxpool.Pool` | ✅ 允许。Service 可持有 pool 执行复杂 SQL（如 alarm engine、weather service） |
| Repository → Service | ❌ **禁止**。Repository 是最底层的数据访问，不得反向调用业务层 |
| Repository → Handler | ❌ **禁止**。完全不允许 |
| Domain ← ANY | Domain 被所有层 import，但 Domain 自身不 import 任何业务包 |
| `internal/` 包之间 | 禁止循环 import（Go 编译器强制）。特别警惕 `gateway/` ↔ `handler/` 之间 |

**判断标准**：如果你发现自己需要 import 一个"更高层"的包来解决问题，说明设计有问题，应该提取接口或重新组织代码。

### 11.2 横切关注点（不可遗漏）

以下横切关注点必须在**每个 HTTP Handler** 上正确配置：

| 关注点 | 规则 | 实现方式 |
|--------|------|---------|
| **认证 (Auth)** | 所有 `/api/v1/*` 路由必须走 JWT 认证 | `middleware.AuthMiddleware(authSvc)` 包装 `protected` mux |
| **授权 (RBAC)** | 非公开路由必须声明所需权限码 | `withPerm(authSvc, "perm.code", handler)` 包装 |
| **限流** | 登录接口必须限流 | `rateLimiter.LimitLogin(handler)` |
| **CORS** | 顶层 mux 必须经过 CORS 中间件 | `middleware.CORS(allowedOrigins, mux)` |
| **日志** | 所有错误路径必须记录 | `slog.Warn/Error("描述", "key", val, "err", err)` |
| **超时** | 任何 TCP/HTTP 外部调用必须有超时 | `context.WithTimeout` 或 `SetReadDeadline` |
| **panic 恢复** | 所有后台 goroutine 必须 `defer recover()` | `safeGo(name, fn)` 包装函数 |

**新增路由检查清单**：

```
□ 路由是否注册在 protected mux 下？（认证）
□ 是否用 withPerm() 包装并传入了正确的权限码？（授权）
□ Handler 中所有 error 是否都有 slog.Warn/Error 记录？（日志）
□ 响应是否通过 ok()/created()/notFound()/serverErr() 统一格式化？
□ 外部调用（如天气 API）是否有超时设置？
```

### 11.3 禁止事项清单

#### 11.3.1 代码层面

| ❌ 禁止行为 | 原因 | 正确做法 |
|-------------|------|---------|
| 用 `_` 丢弃 error | 静默吞错，生产排障无日志 | `slog.Warn("msg", "err", err)` |
| `fmt.Println` / `log.Println` | 与项目统一日志方案不一致 | `slog.Info/Warn/Error` |
| SQL 中字符串拼接用户输入 | SQL 注入风险 | `$1, $2, ...` 占位符 + 白名单校验 |
| 对 `*T` 指针直接解引用而不判 nil | panic 导致进程崩溃 | `if p != nil { use(*p) }` |
| `panic()` 在 HTTP Handler 中 | 会导致整个请求 goroutine 崩溃，无恢复机制 | 返回 error + `serverErr(w, err)` |
| 在 HTTP Handler 中启动无超时的 goroutine | 可能泄漏 | 使用 `context.WithoutCancel(r.Context())` |
| 直接修改已有迁移 SQL 文件 | 破坏 schema 版本一致性 | 新增迁移文件 |
| `go.mod` 中新增非必要依赖 | 增加二进制体积和攻击面 | 优先用标准库，确认必要再加 |
| 在循环中逐行 INSERT | 严重性能问题 | 使用 `pgx.Batch` 批量写入 |
| Handler 方法中写超过 5 行的 SQL | 可读性差、难以测试 | 提取到 Repository 或 Service 层 |
| 新增 Repository/Gateway/Service 代码不写测试 | 核心逻辑无验证，回归风险 | 参照 10.4 节，按模块级别补齐测试 |

#### 11.3.2 架构层面

| ❌ 禁止行为 | 原因 | 正确做法 |
|-------------|------|---------|
| 在 Handler 中写业务逻辑 | 违反分层，难以复用 | 业务逻辑下沉到 Service |
| 跨层绕过中间层调用 | 破坏分层隔离 | 严格遵循 Handler → Service → Repository |
| 创建新模块时跳过 Domain 定义 | 类型不安全，JSON 字段名不一致 | 先定义 `domain.XXX` struct |
| 在测试中依赖真实数据库 | 测试不可重复、不可离线运行 | 使用 `pgxmock` |
| 前端直接操作 `localStorage` 存敏感数据 | token 泄露风险 | 使用 `api/client.ts` 的 token 管理；小程序用 `uni.getStorageSync` |
| 在 `go.sum` 中手动编辑 | 校验和不一致导致构建失败 | `go mod tidy` 自动管理 |

#### 11.3.3 安全层面（从上一轮安全审查固化为强制规则）

| ❌ 禁止行为 | 对应真实漏洞 |
|-------------|-------------|
| CORS 使用 `*` 且不校验请求 Origin（生产环境） | 跨域攻击 |
| JWT 密钥使用默认值 | 任何知道源码的人可伪造 Token |
| 密码重置跳过旧密码验证 | 攻击者知道用户 ID 即可改密码 |
| `interval` 参数直接拼入 SQL 函数名 | SQL 注入 |
| `pathLast`/`pathID` 获取的 ID 不经 >0 校验就传入 SQL | 无效 ID 导致意外行为 |

---

## 12. 规则优先级

当多个规则冲突时，按以下优先级裁决：

```
1. 安全规则（11.3.3）            ← 最高优先级，不可妥协
2. 禁止事项（11.3.1 / 11.3.2）
3. 依赖方向（11.1）
4. 横切关注点（11.2）
5. 编码风格（第 5 节）
6. 命名规范（5.1.1）
7. 其他约定（第 10 节）
```

---

## 13. 项目状态追踪（检查点）

> 最后更新：2026-06-26 12:52

### 13.1 最新进度（Latest Progress）

**后端**（Go 1.25 + PostgreSQL）：
- 58 条 API 路由全部实现，含新增的 `GET /devices/{id}/sensor-data` 和 `PUT /devices/{id}/sensor-config`
- 数据库迁移推进到 v014（新增 `sensor_config` 表 + `device.power_sign` 列 + FK/索引完整性）
- 安全加固：BodyLimit 中间件（1MB）、CORS 白名单、JWT 强制密钥、密码重置强制旧密码、SQL 注入防护
- 大屏 `/screen/data` 项目权限修复：普通用户只看被分配的项目，未分配返回空列表，超管看全部
- 电表 `power_sign` 字段实现总电能加减计算
- orchestrator LogStep 同步更新 DB `done_steps`，前端可看实时执行进度
- config.Load() 返回 `(*Config, error)`，新增 `DBSSLMode`（默认 disable）、`TrustedProxy` 配置

**Web 大屏**（React 19 + Ant Design 6 + TailwindCSS v4）：
- 端口 3001 独立入口，5 秒轮询刷新
- 监控中心：设备拓扑图 + 天气 + 告警 + 能耗统计 + 电能统计（含 power_sign 加减）
- 数据中心：实时数据（设备卡片，温湿度传感器走 sensor-data API）+ 历史数据（Recharts 时序图表 + 开关机时段标注）
- Token 管理：启动时校验 JWT 过期 + 401 自动登出 + `screenClient.ts` 共享 API 客户端
- TailwindCSS v4 设计系统接入（跳过 preflight 避免与 AntD 冲突，深色主题通过 `body.xmeco-dark` 条件激活）

**Web 管理后台**（端口 3000）：
- 15 个页面：Dashboard、Projects、Buildings、Devices、Properties、Registers、Alarms、Logs、StartupPlans、Users、Agents、Permissions、MultiAgent、Login
- 属性配置页：电表设备显示"电能方向"列（+ 加 / - 减），温湿度传感器显示专用 15 列 + 新增/编辑弹窗
- ErrorBoundary 包裹路由，ProtectedRoute 校验 JWT 过期

**小程序**（UniApp Vue 3）：
- 7 个页面：login、index（含天气卡片）、devices、detail（含设备控制确认弹窗）、alarms、history（含日期/指标/间隔筛选）、mine
- API client 有 401 自动重登、超时 15s、clearToken

### 13.2 当前阻碍（Blockers）

| 编号 | 阻碍 | 影响 | 状态 |
|------|------|------|------|
| B-01 | **大屏 5s 轮询对 DB 压力** | `/screen/data` 每次执行 8+ SQL，高频轮询可能拖慢数据库 | 待引入 WebSocket 或服务端缓存 |
| B-02 | **后端需手动重启** | 改 Go 代码后需 `go build` + 重启 server，无热重载 | 可用 `air` 热重载工具解决 |
| B-03 | **server.exe 未重新编译** | 本轮后端改动（power_sign/sensor_config/权限修复）在 `server.exe` 中不存在，start.bat 用的是旧 exe | 需执行 `go build -o server.exe ./cmd/server/main.go` |
| B-04 | **DataCenter N+1 请求** | 实时数据面板对每个设备单独请求 `/properties`，20+ 设备时并发压力 | 待后端新增批量接口 |

### 13.3 未完事项标记（Placeholders & Hardcoded）

⚠️ 以下接口/页面/数据处于占位符或硬编码状态，后续会话须注意：

| 位置 | 类型 | 说明 |
|------|------|------|
| `Screen.tsx` TABS 中 `maintain`/`task`/`decision`/`logs`/`energy` | 占位符 | 大屏 5 个 Tab 显示"开发中"，未实现实际功能 |
| `DataCenter.tsx` RealtimePanel | 硬编码 | 设备分组顺序 `DATA_ORDER` 和颜色 `DATA_COLORS` 硬编码在文件中 |
| `DataCenter.tsx` HistoryPanel | 硬编码 | 图表颜色 `CHART_COLORS` 硬编码 8 色循环 |
| `dashboard.go` 能效计算 | 硬编码 | `savingRate * 0.58` 中的 0.58 是碳排放系数，硬编码 |
| `efficiency.go` `estimatePower()` | 硬编码 | 各设备类型的额定功率估算值硬编码（主机 120kW、泵 30kW 等） |
| `efficiency.go` `generateDemoEfficiency()` | 演示数据 | 无设备时返回 8 条硬编码 demo 数据，DeviceID=0 |
| `predict.go` `officeProfile` | 硬编码 | 24h 负荷曲线硬编码为办公楼 profile，不区分酒店/商场 |
| `weather.go` wttr.in | 外部依赖 | 天气数据依赖免费 wttr.in API，无 API Key，可能被限流 |
| `miniapp/pages/history` | 未关联路由 | history.vue 已实现但未在 pages.json 中注册为 tabBar 页面 |
| `000014_sensor_config.up.sql` | 死列 | `sensor_type` 列已建但 SaveSensorConfig 不写入、前端已移除该字段 |
| `TrustedProxy` 配置 | 部分实现 | config.go 有 `TrustedProxyCIDRs()` 并传给 RateLimiter，但 AuthMiddleware 中获取真实 IP 的逻辑未确认 |

### 13.4 下一步计划（Next Steps）

| 优先级 | 任务 | 说明 |
|--------|------|------|
| **P0** | 重新编译 `server.exe` | `go build -o server.exe ./cmd/server/main.go`，让 start.bat 用上新代码 |
| **P0** | 大屏维保中心 Tab | 实现设备维保记录、巡检计划、故障工单 |
| **P0** | 大屏任务中心 Tab | 实现启停计划执行监控（实时步骤进度 + 日志） |
| **P1** | WebSocket 实时推送 | 替换 5s 轮询，后端 poller 写入遥测后广播到 WS 连接 |
| **P1** | 告警通知渠道 | 短信/公众号/邮件，基于 `alarm_rule.notify_users` |
| **P1** | 设备控制闭环 | Modbus 写后读回确认 |
| **P1** | 批量属性查询接口 | `/properties/batch?device_ids=1,2,3` 解决 N+1 |
| **P2** | 大屏能耗中心 Tab | 能效分析报告、COP 趋势、节能量核算 |
| **P2** | 大屏决策中心 Tab | 智能分析策略展示 + 一键执行 |
| **P2** | 时序数据分区 | TimescaleDB hypertable 或 date partition |
| **P2** | 多级权限数据隔离 | Agent 级项目/楼宇/设备可见性 |
| **P3** | Repository 泛型重构 | 消除 CRUD 重复代码 |
| **P3** | 核心模块测试补全 | gateway 协议解析 + poller 解码 + alarm 引擎 |
