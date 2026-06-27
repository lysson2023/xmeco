# XMECO 测试套件 (Test Harness)

> 基于代码库全量分析生成。覆盖所有关键逻辑路径，按模块分层组织。
> 每条用例标注：被测函数、场景、输入条件、预期行为、Mock 需求、优先级。

---

## 0. 工具与 Mock 总览

| 依赖 | Mock 工具 | 适用层 |
|---|---|---|
| PostgreSQL | `pgxmock/v4` (`NewPool`) | Repository, Service (告警/智能) |
| HTTP Handler | `httptest.NewRecorder` + `httptest.NewRequest` | Handler, Middleware |
| JWT | 直接构造 `auth.Claims` + `jwt.NewWithClaims` | Middleware, Handler |
| 外部 HTTP (天气) | `httptest.NewServer` 替代 wttr.in | Weather Service |
| 网关 Transport | 实现 `transport.Transport` 接口的 mock | Poller, Manager |
| 时间 | `time.Now()` 直接调用 → 需近似断言 | Alarm, Weather Cache, Scheduler |
| 嵌入迁移 | `embed.FS` / 测试用临时 SQL 文件 | Migration |

---

## 1. Config 层 (`internal/config/config.go`)

### 路径 1.1: JWT 密钥安全检查

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| C-01 | 生产环境默认密钥拒绝启动 | `XMECO_JWT_SECRET`=`xmeco-dev-secret-change-in-production`, `XMECO_DEV_MODE` 未设置 | `Load()` 返回 `ErrNoSecret` |
| C-02 | 开发模式默认密钥允许启动 | `XMECO_JWT_SECRET`=`xmeco-dev-secret-...`, `XMECO_DEV_MODE`=`true` | `Load()` 成功，不返回 `ErrNoSecret` |
| C-03 | 自定义密钥正常启动 | `XMECO_JWT_SECRET`=`my-production-secret` | `Load()` 成功，`cfg.JWTSecret == "my-production-secret"` |

### 路径 1.2: 环境变量解析

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| C-04 | 全部默认值 | 不设置任何环境变量 | `Port=9090`, `DBHost=localhost`, `RetentionDays=730`, `PollIntervalSec=3` |
| C-05 | 自定义端口 | `XMECO_SERVER_PORT=8080` | `cfg.ServerPort == 8080` |
| C-06 | 负数回退 | `XMECO_RETENTION_DAYS=-5` | `cfg.RetentionDays == 730` (默认值回退) |
| C-07 | 非法整数回退 | `XMECO_POLL_INTERVAL_SEC=abc` | `cfg.PollIntervalSec == 3` (默认值回退 + Warn 日志) |
| C-08 | 显式零保留 | `XMECO_RETENTION_DAYS=0` | `cfg.RetentionDays == 0` (不清理) |

### 路径 1.3: DSN 与安全

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| C-09 | DSN 构建 | 标准配置 | DSN 包含 `user=postgres password=xmeco123 host=localhost` |
| C-10 | MaskedDSN 脱敏 | 标准配置 | `MaskedDSN()` 输出含 `password=***`，不含真实密码 |
| C-11 | 密码含特殊字符 | `XMECO_DB_PASSWORD=a!b@c#` | DSN 中密码被 `url.QueryEscape` 正确编码 |

### 路径 1.4: TrustedProxy

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| C-12 | 空 TrustedProxy | `XMECO_TRUSTED_PROXY=""` | `TrustedProxyCIDRs()` 返回 nil |
| C-13 | 单 CIDR | `XMECO_TRUSTED_PROXY=10.0.0.0/8` | 解析为 1 个 CIDR |
| C-14 | 多 CIDR | `XMECO_TRUSTED_PROXY=10.0.0.0/8, 172.16.0.0/12` | 解析为 2 个 CIDR |

---

## 2. Repository 层 (`internal/repository/postgres/`)

> Mock 方式: `pgxmock.NewPool()` → `defer mock.Close()` → `mock.ExpectQuery/Exec(...).WithArgs(...).WillReturnRows/Result(...)`

### 2.1 BuildingRepo — 缺失覆盖优先

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| R-01 | **GetByID 未找到** | `ExpectQuery(...).WithArgs(999).WillReturnError(pgx.ErrNoRows)` | `(nil, nil)` — 不是 error |
| R-02 | GetByID 数据库异常 | `ExpectQuery(...).WithArgs(1).WillReturnError(fmt.Errorf("conn refused"))` | `(nil, error)` — error 非 nil |
| R-03 | List 空结果 | `ExpectQuery(...).WillReturnRows(空Rows)` | `([]domain.Building{}, nil)` — 空切片非 nil |
| R-04 | Create 含 nil 字段 | `outdoor_temp=nil`, `outdoor_humidity=nil` | INSERT 成功，`pgxmock.AnyArg()` 匹配 nil 指针 |

### 2.2 DeviceRepo — 大量缺失

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| R-05 | **Create** 基本写入 | `ExpectQuery("INSERT INTO device...").WithArgs(...).WillReturnRows(id=1)` | `(1, nil)` |
| R-06 | **Update** 全字段 | `ExpectExec("UPDATE device SET...").WithArgs(...).WillReturnResult(UPDATE 1)` | `nil` (无错误) |
| R-07 | **Delete** 存在 | `ExpectExec("DELETE FROM device...").WithArgs(1).WillReturnResult(DELETE 1)` | `nil` |
| R-08 | Delete 不存在 | `ExpectExec(...).WillReturnResult(DELETE 0)` | `nil` (幂等) |
| R-09 | **GetByID 未找到** | `ExpectQuery(...).WillReturnError(pgx.ErrNoRows)` | `(nil, nil)` |
| R-10 | List 含可空字段扫描 | `gateway_imei=NULL, rated_voltage=NULL, last_online_at=NULL` | COALESCE 转为零值 |

### 2.3 PropertyRepo — 缺失覆盖

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| R-11 | **GetByID** 正常 | `ExpectQuery(...).WillReturnRows(1行)` | `(*Property, nil)` |
| R-12 | **GetByID 未找到** | `ExpectQuery(...).WillReturnError(pgx.ErrNoRows)` | `(nil, nil)` |
| R-13 | **Update** 基本 | `ExpectExec("UPDATE device_properties...").WillReturnResult(UPDATE 1)` | `nil` |
| R-14 | **Delete** 基本 | `ExpectExec("DELETE...").WillReturnResult(DELETE 1)` | `nil` |

### 2.4 RegisterRepo — 缺失覆盖

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| R-15 | **GetByID** 正常 | `ExpectQuery(...).WillReturnRows(1行)` | `(*Register, nil)` |
| R-16 | **GetByID 未找到** | `WillReturnError(pgx.ErrNoRows)` | `(nil, nil)` |
| R-17 | **Create** 含可空字段 | `write_addr=nil, write_code=nil, command_name=nil` | INSERT 成功 |
| R-18 | **Delete** 正常 | `ExpectExec(...).WillReturnResult(DELETE 1)` | `nil` |
| R-19 | ListByDeviceID JOIN | `ExpectQuery(...JOIN device_properties...).WillReturnRows(2行)` | 2 条 Register |

### 2.5 ProjectRepo — 缺失事务路径

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| R-20 | **GetByID 未找到** | `WillReturnError(pgx.ErrNoRows)` | `(nil, nil)` — 同 Building #R-01 |
| R-21 | **SetProjectUsers** 完整事务 | `ExpectBegin → ExpectExec(DELETE) → ExpectExec(INSERT)*N → ExpectCommit` | `nil` |
| R-22 | SetProjectUsers 空列表 | userIDs=[] | 仅 DELETE，无 INSERT 循环 |
| R-23 | SetProjectUsers 事务失败回滚 | `ExpectBegin → ExpectExec(DELETE).WillReturnError(...)` → `ExpectRollback` | error 非 nil |
| R-24 | **ListProjectUsers** 正常 | `ExpectQuery(...).WillReturnRows(3行)` | `([]int{1,2,3}, nil)` |
| R-25 | ListProjectUsers 空 | `ExpectQuery(...).WillReturnRows(空)` | `([]int{}, nil)` |

### 2.6 AdminRepo — 缺失方法

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| R-26 | **GetPasswordHash** 正常 | `ExpectQuery("SELECT password_hash...").WillReturnRows` | `("$2a$...", nil)` |
| R-27 | GetPasswordHash 用户不存在 | `WillReturnError(pgx.ErrNoRows)` | `("", pgx.ErrNoRows)` |
| R-28 | **DBStats** 正常 | 4 次 `ExpectQuery` 分别返回行 | 4 个字段均非零 |
| R-29 | DBStats 部分查询失败 | 第 3 个 `QueryRow.Scan` 失败 | 返回 error |
| R-30 | ListUsers 空 | `ExpectQuery(...).WillReturnRows(空)` | `nil, nil` |

---

## 3. Service 层 — 业务逻辑

### 3.1 Auth Service (`internal/service/auth/`)

> **注意**: `Login` 和 `HasPermission` 直接持有 `*pgxpool.Pool`，不可用 pgxmock 测试。需重构为 DBTX 接口或做集成测试。

#### JWT 签发/验证 (纯函数，可测试)

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| A-01 | ValidateToken 正常 | 用 `jwtSecret` 签发含 `uid=1, uname=admin, role=super_admin` 的 token | `Claims{UserID:1, Username:"admin", RoleCode:"super_admin"}` |
| A-02 | ValidateToken 过期 | 签发 `exp=now-1h` 的 token | 返回 error（含 "expired"） |
| A-03 | ValidateToken 错误密钥 | 用 `wrong-secret` 签发的 token | 返回 error |
| A-04 | ValidateToken 格式错误 | `"not.a.jwt"` | 返回 error |
| A-05 | ValidateToken 空字符串 | `""` | 返回 error |

#### bcrypt (静态方法)

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| A-06 | HashPassword 正常 | `"admin123"` | 返回 60 字符 bcrypt 哈希，以 `$2a$10$` 开头 |
| A-07 | HashPassword 空密码 | `""` | 返回有效哈希（bcrypt 允许空） |
| A-08 | CheckPassword 匹配 | 哈希 + 正确密码 | `true` |
| A-09 | CheckPassword 不匹配 | 哈希 + 错误密码 | `false` |

#### Login (需要真实 DB 或 DBTX 重构后)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| A-10 | Login 成功 | AdminRepo 返回用户（含 password_hash），bcrypt 匹配，权限查询返回列表 | 返回 token + `*User{Permissions:[...]}` |
| A-11 | Login 用户名不存在 | `QueryRow` 返回 `pgx.ErrNoRows` | `ErrInvalidCredentials` |
| A-12 | Login 密码错误 | bcrypt 不匹配 | `ErrInvalidCredentials` |
| A-13 | Login 用户已禁用 | `is_active=false` | `ErrUserInactive` |
| A-14 | Login 数据库异常 | `QueryRow` 返回连接错误 | `ErrInternal` |

#### RBAC

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| A-15 | HasPermission 有权限 | `SELECT COUNT(*)` 返回 1 | `(true, nil)` |
| A-16 | HasPermission 无权限 | `SELECT COUNT(*)` 返回 0 | `(false, nil)` |
| A-17 | HasPermission DB 异常 | `QueryRow` 返回连接错误 | `(false, error)` |

### 3.2 Alarm Engine (`internal/service/alarm/engine.go`)

#### triggered() 纯函数 — 6 种条件类型

| ID | 场景 | cond | val | threshold | minVal | maxVal | 预期 |
|---|---|---|---|---|---|---|---|
| AL-01 | gt 触发 | `"gt"` | 30.0 | 25.0 | - | - | `true` |
| AL-02 | gt 不触发 | `"gt"` | 25.0 | 25.0 | - | - | `false` (等于不触发) |
| AL-03 | ge 触发 | `"ge"` | 25.0 | 25.0 | - | - | `true` (等于触发) |
| AL-04 | lt 触发 | `"lt"` | 10.0 | 15.0 | - | - | `true` |
| AL-05 | le 触发 | `"le"` | 15.0 | 15.0 | - | - | `true` |
| AL-06 | eq 触发 | `"eq"` | 50.0 | 50.0 | - | - | `true` |
| AL-07 | eq 不触发 | `"eq"` | 50.1 | 50.0 | - | - | `false` |
| AL-08 | range 低于下限 | `"range"` | 5.0 | - | `"10"` | `"30"` | `true` (在范围外) |
| AL-09 | range 高于上限 | `"range"` | 35.0 | - | `"10"` | `"30"` | `true` |
| AL-10 | range 在范围内 | `"range"` | 20.0 | - | `"10"` | `"30"` | `false` |
| AL-11 | range 边界等于下限 | `"range"` | 10.0 | - | `"10"` | `"30"` | `false` (闭区间) |
| AL-12 | range 仅上限 | `"range"` | 35.0 | - | `""` | `"30"` | `true` (无下限则上限生效) |
| AL-13 | range 解析失败跳过 | `"range"` | 5.0 | - | `"abc"` | `"30"` | `false` (minVal 无效→跳过规则) |

#### Evaluate() — 需要 pgxmock

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| AL-14 | 规则触发 + 去重插入 | 查询返回 1 条规则，值超过阈值 | `INSERT ... ON CONFLICT DO NOTHING`，`RowsAffected>0`，无错误 |
| AL-15 | 规则触发但已有未确认告警 | 同上，但 `ON CONFLICT` 跳过 | `RowsAffected==0`，无错误（去重生效） |
| AL-16 | 无匹配规则 | 查询返回空行 | 无 INSERT，无错误 |
| AL-17 | 规则查询 DB 错误 | `ExpectQuery` 返回错误 | 返回 error |
| AL-18 | 多规则部分触发 | 3 条规则，仅第 2 条触发 | 仅触发 1 次 INSERT |
| AL-19 | device_type NULL 通配 | 规则 `device_type IS NULL` | 匹配所有设备类型 |

#### AlertOffline()

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| AL-20 | 离线告警写入 | 正常设备 | `INSERT ... ON CONFLICT (device_id, alarm_type) WHERE ack_at IS NULL DO NOTHING` |
| AL-21 | 已存在离线告警去重 | 已有未确认离线告警 | `RowsAffected==0`，无错误 |

#### 辅助函数

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| AL-22 | condCN 已知条件 | `"gt"` | `"大于"` |
| AL-23 | condCN 未知条件 | `"xyz"` | `"xyz"` (原样返回) |
| AL-24 | parseFloatOK 有效 | `"3.14"` | `(3.14, true)` |
| AL-25 | parseFloatOK 无效 | `"abc"` | `(0, false)` |

### 3.3 Intelligence Service (`internal/service/intelligence/`)

#### 湿球温度 (纯函数)

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| I-01 | 标准条件 | `dryBulb=30°C, rh=60%` | 湿球 ~23.3°C (±0.5) |
| I-02 | 边界: 低限 | `dryBulb=5°C, rh=10%` | 湿球 < dryBulb (单调性质) |
| I-03 | 边界: 高限 | `dryBulb=35°C, rh=90%` | 湿球 < dryBulb |
| I-04 | 零湿度 | `dryBulb=25°C, rh=0%` | 湿球 < dryBulb (保守) |
| I-05 | 饱和 | `dryBulb=25°C, rh=100%` | 湿球 ≈ 干球 |

#### 能效分析 (`analysis.go`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| I-06 | 设备有遥测数据 | 返回功率数据的遥测查询 | 效率计算 >= 0，非 NaN/Inf |
| I-07 | 设备无遥测数据 | 遥测查询返回空 | 回退到额定功率估算 |
| I-08 | 无任何设备 | 设备查询返回空 | 返回演示数据 (DeviceID=0) |
| I-09 | NaN/Inf 防护 | 遥测数据含 +Inf | `math.IsInf` 检测 → 跳过，不 panic |
| I-10 | chillerPartLoadFactor >=70% | `loadPct=75` | `0.98` |
| I-11 | chillerPartLoadFactor 30-50% | `loadPct=40` | `0.88` |
| I-12 | chillerPartLoadFactor <30% | `loadPct=20` | `0.75` |
| I-13 | pumpEfficiencyScore 最佳区间 | `score=72` (60-85 区间) | 高分 |
| I-14 | pumpEfficiencyScore 低于区间 | `score=30` (<60) | 低分 |

#### 负荷预测 (`predict.go`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| I-15 | 高温日 (>=22°C) | `outdoorTemp=35` | 预测负荷 > 基础负荷 × 2.0 |
| I-16 | 低温日 (<=18°C) | `outdoorTemp=10` | 预测负荷 = 基础负荷 × 0.6 |
| I-17 | 中温线性插值 | `outdoorTemp=20` | 预测负荷在 0.6~1.0 倍基础负荷之间 |
| I-18 | DB 无 chiller 额定功率 | 设备查询返回空 | 基础负荷回退到 200kW |
| I-19 | 输出长度 | 任意温度 | 返回 24 个数据点 |

#### 电能质量 (`power_quality.go`)

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| I-20 | 电压优 | 电压偏差 <5%, 合格率 100% | GradeVoltage = "优" |
| I-21 | 电压偏差良 | 偏差 6%, 合格率 90% | GradeVoltage = "良" |
| I-22 | 电压偏差差 | 偏差 8% | GradeVoltage = "差" |
| I-23 | 电流不平衡优 | <=10% | GradeCurrent = "优" |
| I-24 | 功率因数优 | PF=0.97, 合格率 95% | GradePF = "优" |
| I-25 | 频率偏差优 | 偏差 0.3% | GradeFreq = "优" |
| I-26 | 谐波优 | max(THDV,THDI) <= 3% | GradeHarmonic = "优" |
| I-27 | overallGrade 全优 | 全部 5 项为优 | "优" |
| I-28 | overallGrade 一项差 | 1 项差 + 其余优 | "差" (差优先) |

#### 泵/电价/轮换

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| I-29 | 泵优化触发 | 泵频率 <47Hz, DeltaT 大 | 建议降频，estimatePowerKWSaved > 0 |
| I-30 | 泵优化未触发 | 频率 = 49Hz | 无建议或节电=0 |
| I-31 | 电价高峰时段 | 当前时间 10:00 (峰) | 建议减少运行或转储冷 |
| I-32 | 电价低谷时段 | 当前时间 02:00 (谷) | 建议蓄冷 |
| I-33 | 设备轮换单台 | 同类型仅 1 台设备 | "无需轮换" |
| I-34 | 设备轮换多台 | 同类型 3 台设备 | 按运行小时排序: 主机→备机→停机 |

### 3.4 Weather Service (`internal/service/external/weather/`)

#### JSON 解析 (纯函数)

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| W-01 | wttrParse 正常 | 标准 wttr.in JSON | 返回 `*WttrCondition`，字段非空 |
| W-02 | wttrParse 空条件数组 | `current_condition:[]` | 返回 error |
| W-03 | wttrParse 中文城市 | 深圳返回含 "Shower" 的响应 | 翻译为 "阵雨" |
| W-04 | wttrParse 非法 JSON | `"{not json}"` | 返回 error |

#### 缓存逻辑 (需要 pgxmock)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| W-05 | 缓存命中 | `SELECT ... WHERE expires_at > NOW()` 返回行 | 直接返回缓存数据，不调 HTTP |
| W-06 | 缓存未命中 → 远程成功 | 缓存查询空 + HTTP 200 | 调 wttr.in，结果写入缓存 (TTL=60min) |
| W-07 | 缓存未命中 → 远程失败 → 过期缓存回退 | 缓存查询空 + HTTP 失败 + 过期缓存存在 | 返回过期缓存数据 (降级) |
| W-08 | 缓存未命中 → 远程失败 → 无任何缓存 | 缓存查询空 + HTTP 失败 + 过期缓存也无 | 返回原始 error |
| W-09 | cityID <= 0 跳过写缓存 | `cityID=0` | 不调用 INSERT INTO weather_cache |

#### 天气翻译 (纯函数)

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| W-10 | translateWeather 精确匹配 | `"Clear"` | `"晴"` |
| W-11 | translateWeather 代码匹配 | `text="", code="113"` | `"晴"` |
| W-12 | translateWeather 前缀回退 | `text="", code="302"` (3xx) | `"阵雨"` |
| W-13 | translateWeather 全部不匹配 | `text="UnknownXxx", code="999"` | `"多云"` (最终回退) |

### 3.5 Telemetry Poller (`internal/service/telemetry/poller.go`)

#### decodeVal (纯函数，核心)

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| T-01 | 空数据 | `data=[]byte{}` | `0.0` |
| T-02 | 单字节无符号 | `data=[0x2A], type="u8"` | `42.0` |
| T-03 | 双字节大端无符号 | `data=[0x01,0x00], type="u16"` | `256.0` |
| T-04 | 双字节小端 (低位在前) | `data=[0x00,0x01], order="低位在前", type="u16"` | `256.0` |
| T-05 | 四字节大端无符号 | `data=[0x00,0x00,0x01,0x00], type="u32"` | `256.0` |
| T-06 | 四字节低字在前 | `data=[0x00,0x01,0x00,0x00], order="低字在前", type="u32"` | `256.0` |
| T-07 | IEEE 754 浮点 | `data=[0x42,0x48,0x00,0x00]` (50.0f) | `50.0` |
| T-08 | 有符号 s16 负数 | `data=[0xFF,0xCE], type="s16"` | `-50.0` |
| T-09 | 有符号 s32 负数 | `data=[0xFF,0xFF,0xFF,0xCE], type="s32"` | `-50.0` |
| T-10 | 掩码提取 | `data=[0x12,0x34], mask="00FF"` | 仅低字节 `52.0` |
| T-11 | 倍率除法 | `data=[0x03,0xE8], magnification=10.0` | `100.0` (1000/10) |
| T-12 | 倍率为零 | `data=[0x01,0x00], magnification=0` | 不应 panic (除以零防护) |

#### parseStatusMapping

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| T-13 | 匹配 | `statusCode="01=运行,02=停机", rawVal=1.0` → hex "01" | `"运行"` |
| T-14 | 不匹配 | `statusCode="01=运行", rawVal=3.0` → hex "03" | `""` |
| T-15 | 大小写不敏感 | `statusCode="0A=告警", rawVal=10.0` → hex "0a" | `"告警"` |

#### reorderForOrder

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| T-16 | 大端 (空/默认) | `data=[0x01,0x02,0x03,0x04], order=""` | 不变 |
| T-17 | 低位在前 | `data=[0x01,0x02,0x03,0x04]` | `[0x02,0x01,0x04,0x03]` |
| T-18 | 低字在前 | `data=[0x01,0x02,0x03,0x04]` | `[0x03,0x04,0x01,0x02]` |

### 3.6 Migration (`internal/service/migration/`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| M-01 | 全新数据库，从头迁移 | `schema_migrations` 为空 | 执行全部迁移 (001→最新) |
| M-02 | 增量迁移 (跳过已应用) | `schema_migrations` 含 001-005 | 仅执行 ≥006 |
| M-03 | detectExistingVersion 检测到 users 表 | `users` 表存在 | 返回 1 (版本 001 已存在) |
| M-04 | detectExistingVersion 检测到 role 表 | `role` 表存在，无 users | 返回 2 |
| M-05 | detectExistingVersion 全部匹配 (最新) | 所有 11 个检测点均存在 | 返回对应版本号 |
| M-06 | detectExistingVersion 全不匹配 | 空数据库 | 返回 0 |
| M-07 | 迁移文件含 UTF-8 BOM | 文件以 `\xEF\xBB\xBF` 开头 | BOM 被剥离，SQL 正常执行 |
| M-08 | 单个迁移失败 (事务回滚) | 第 N 个迁移 SQL 语法错误 | 该迁移事务回滚，之前已应用的保留，之后未执行 |
| M-09 | 幂等: 重复 RUN | 所有迁移已标记 | 无任何新执行 |

---

## 4. Gateway 层 — 协议解析

### 4.1 Modbus (`internal/gateway/modbus/modbus.go`)

#### CRC16

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| G-01 | 标准 CRC 向量 | `[0x01,0x03,0x00,0x00,0x00,0x01]` | `0x840A` |
| G-02 | 空数据 | `[]byte{}` | `0xFFFF` (初始值不变) |
| G-03 | 单字节 | `[0x00]` | 非零 |

#### BuildReadCommand

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| G-04 | 标准读保持寄存器 | `devAddr=1, funcCode=3, startAddr=0, count=1` | 8 字节帧，末尾 2 字节为 CRC |
| G-05 | 多寄存器读取 | `addr=100, count=10` | 数量字段 = `0x000A` |

#### BuildWriteCommand

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| G-06 | 单寄存器写入 (06) | `devAddr=1, funcCode=6, addr=0, count=1, value=0xFF00` | 8 字节帧 |
| G-07 | 多寄存器写入 (10) | `funcCode=0x10, count=2` | 动态长度 = 9 + dataLen |
| G-08 | 默认回退 | `funcCode=0x99` (未知) | 使用 `BuildWriteSingleCommand` |

#### BuildWriteSingleCommand / BuildWriteMultiCommand

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| G-09 | 写单个线圈 (05) | `addr=0, value=0xFF00` | 8 字节帧，funcCode=0x05 |
| G-10 | 写多个寄存器 (10) | `addr=0, count=2, data=[4]byte` | 帧含 byteCount=4 |

#### ParseResponse

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| G-11 | 解析读响应 (01-04) | `[addr, 0x03, byteCount, data..., crcL, crcH]` | `(data, true)` |
| G-12 | 解析写单响应 (05-06) | `[addr, 0x06, addrH, addrL, valH, valL, crcL, crcH]` | `(bytes[2:6], true)` |
| G-13 | 解析写多响应 (10) | `[addr, 0x10, addrH, addrL, countH, countL, crcL, crcH]` | `(bytes[4:6], true)` |
| G-14 | 校验失败 | CRC 最后两字节被破坏 | `(nil, false)` |
| G-15 | 响应太短 | `[0x01, 0x03]` (2 字节) | `(nil, false)` |
| G-16 | 未知功能码 | `funcCode=0x2B` | `(nil, false)` |
| G-17 | 异常响应 (0x80+) | `[addr, 0x83, excCode, crcL, crcH]` (不按长度规则) | 预期失败 (当前未处理异常码) |

#### VerifyCRC

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| G-18 | CRC 正确 | 有效 Modbus 帧 | `true` |
| G-19 | CRC 错误 | 最后字节翻转 | `false` |
| G-20 | 太短 | `[0x01]` (1 字节) | `false` |

#### CodeFromStr

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| G-21 | 已知码 | `"03"` | `0x03` |
| G-22 | 未知码 | `"FF"` | `0x06` (默认回退) |

### 4.2 Custom Transport (`internal/gateway/transport/custom.go`)

#### wrap (帧构建)

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| CT-01 | 基本包裹 | `data=[0x01,0x03,0x00,0x00,0x00,0x01]` | 帧以 `0x68` 开头，以 `0x16` 结尾，长度 ≥14 |
| CT-02 | 空数据 | `data=[]` | 最小帧 = 14 字节 (头部11 + 校验和2 + 尾部1) |
| CT-03 | 校验和正确性 | 任意数据 | 手动计算帧字节和 (不含最后3字节) == 解码校验和字段值 |

#### unwrap (帧解析)

| ID | 场景 | 输入 | 预期 |
|---|---|---|---|
| CT-04 | 正常解包 | `wrap(somePayload)` 的输出 | 返回原始 payload |
| CT-05 | 帧太短 | `<14` 字节 | error |
| CT-06 | 起始标记错 | 帧 `[0x00, ...]` 不以 0x68 开头 | error |
| CT-07 | 结束标记错 | 帧不以 0x16 结尾 | error |
| CT-08 | 命令字节不匹配 | 帧中 `customCmdHi/customCmdLo` 被修改 | error |
| CT-09 | 声明长度与缓冲区不一致 | 长度字段声称 100 但实际缓冲区短 | error |
| CT-10 | 校验和错 | 帧中某字节翻转 | error |
| CT-11 | 最大帧 | `data` 接近 1024 字节 (读取缓冲区限制) | 正常解包 |

### 4.3 Transparent Transport (`internal/gateway/transport/transparent.go`)

| ID | 场景 | 行为 | 预期 |
|---|---|---|---|
| DT-01 | 正常收发 | 发送 Modbus 读命令，收到有效响应 | 返回解析后数据 |
| DT-02 | CRC 校验失败 | 响应 CRC 错误 | 返回 error |
| DT-03 | 功能码 01-04 响应长度计算 | 响应 `[addr, 0x03, 6, data..., crc]` | 预期总长度 = 3+6+2=11 |
| DT-04 | 功能码 05/06 固定长度 | 响应写确认 | 预期总长度 = 8 |
| DT-05 | 功能码 0x10 固定长度 | 响应写确认 | 预期总长度 = 8 |
| DT-06 | 错误响应 (0x80 掩码) | `[addr, 0x83, exc, crc]` | 预期总长度 = 5 |
| DT-07 | 未知功能码 | 响应 funcCode=0x2B | 返回 error |
| DT-08 | 读取超时 (10s 截止) | 从不发送响应 | 返回超时 error |
| DT-09 | 排空循环含陈旧数据 | 连接缓冲区有上次残留字节 | Drain 清除后正常读取 |
| DT-10 | 排空循环 10 次上限 | 不断收到陈旧数据 | 最多排空 10 次后继续 |

---

## 5. Middleware 层

### 5.1 AuthMiddleware (`internal/api/middleware/auth.go`)

| ID | 场景 | 请求 | 预期 |
|---|---|---|---|
| MW-01 | 有效 Token | `Authorization: Bearer <valid>` | 200，Context 含 Claims |
| MW-02 | 无 Authorization 头 | 无 Authorization | 401 + JSON `{"error":"未提供认证令牌"}` |
| MW-03 | 错误前缀 | `Authorization: Basic xxx` | 401 |
| MW-04 | 过期 Token | `Bearer <expired>` | 401 |
| MW-05 | 格式错误 Token | `Bearer not.a.token` | 401 |
| MW-06 | 空 Token 值 | `Authorization: Bearer ` | 401 |
| MW-07 | GetClaims 无上下文 | `context.Background()` 的 request | 返回 nil |
| MW-08 | GetClaims 类型错误 | 上下文中存了非 *Claims 的值 | 返回 nil |

### 5.2 CORS (`internal/api/middleware/cors.go`)

| ID | 场景 | 请求 | 预期 |
|---|---|---|---|
| MW-09 | 匹配的来源 | `Origin: http://localhost:3000`, 白名单含该域名 | `Access-Control-Allow-Origin: http://localhost:3000` + `Access-Control-Allow-Credentials: true` |
| MW-10 | 不匹配的来源 | `Origin: http://evil.com`, 白名单不含 | 无 CORS 头 (浏览器阻止) |
| MW-11 | 通配符 + 凭证头 | `Origin: http://any.com`, 白名单=`*`, 含 Authorization | 回显 `http://any.com` (非 `*`)，`Vary: Origin` |
| MW-12 | 通配符 + 无凭证 | `Origin: http://any.com`, 白名单=`*`, 无 Authorization | `Access-Control-Allow-Origin: *` |
| MW-13 | OPTIONS 预检 | `OPTIONS /api/v1/foo` | 204 No Content |
| MW-14 | 无 Origin 头 | 无 Origin 头 | 使用白名单第一个域名作为 ACAO |

### 5.3 RateLimiter (`internal/api/middleware/ratelimit.go`)

| ID | 场景 | 行为 | 预期 |
|---|---|---|---|
| MW-15 | 限制内通过 | 第 1-10 次请求/分钟 | 200 |
| MW-16 | 超限拦截 | 第 11 次请求/分钟 (默认限制 10) | 429 + `Retry-After` 头 |
| MW-17 | 不同 IP 独立计数 | IP-A 第 11 次，IP-B 第 1 次 | IP-A:429, IP-B:200 |
| MW-18 | 窗口重置 | 等待 1 分钟后再次请求 | 200 |
| MW-19 | 受信代理 X-Forwarded-For | `X-Forwarded-For: 1.2.3.4`, RemoteAddr 在受信 CIDR | 用 `1.2.3.4` 计数 |
| MW-20 | 受信代理 X-Real-IP | `X-Real-IP: 5.6.7.8`, 无 X-Forwarded-For | 用 `5.6.7.8` 计数 |
| MW-21 | 非受信代理 | X-Forwarded-For 存在但 RemoteAddr 不在受信 CIDR | 用 RemoteAddr 计数 |
| MW-22 | 并发安全 | 50 goroutine 并发请求 | 无竞态，计数准确 |

### 5.4 RBAC (`internal/api/middleware/rbac.go`)

| ID | 场景 | Claims | 预期 |
|---|---|---|---|
| MW-23 | super_admin 绕过 | `RoleCode="super_admin"` | 直接通过，不调 `HasPermission` |
| MW-24 | 有权限 | `UserID=2, RoleCode="admin"`, HasPermission→true | 200 |
| MW-25 | 无权限 | `UserID=2`, HasPermission→false | 403 |
| MW-26 | 无 Claims | Context 无 Claims | 401 |
| MW-27 | DB 异常 | HasPermission 返回 error | 503 |

### 5.5 BodyLimit (`internal/api/middleware/bodylimit.go`)

| ID | 场景 | 请求体 | 预期 |
|---|---|---|---|
| MW-28 | 在限制内 | 500KB, BodyLimit=1MB | 200 |
| MW-29 | 超限 | 2MB, BodyLimit=1MB | 请求体读取失败 (MaxBytesReader 阻断) |

---

## 6. Handler 层 (`internal/api/handler/`)

> Mock 方式: `httptest.NewRecorder()` + `httptest.NewRequest()` + mock Service/Repo

### 6.1 辅助函数 (`models.go`)

| ID | 场景 | 请求 | 预期 |
|---|---|---|---|
| H-01 | pathID 正常 | URL `/api/v1/foo/42` + `r.SetPathValue("id","42")` | `42` |
| H-02 | pathID 非数字 | URL `/api/v1/foo/abc` | `0` |
| H-03 | pathID 空 | 无 path value | `0` |
| H-04 | pathID 大数 | URL `/api/v1/foo/999999999` | `999999999` |
| H-05 | queryInt 正常 | `?device_id=5` | `5` |
| H-06 | queryInt 缺失 | 无参数 | `0` (默认) |
| H-07 | queryInt 非数字 | `?device_id=abc` | `0` (静默) |
| H-08 | ok() 响应 | 调用 `ok(w, map[string]any{"a":1})` | 200, `Content-Type: application/json`, `{"a":1}` |
| H-09 | created() 响应 | 调用 `created(w, data)` | 201 |
| H-10 | notFound() 响应 | 调用 `notFound(w, "X不存在")` | 404, `{"error":"X不存在"}` |
| H-11 | serverErr() 响应 | 调用 `serverErr(w, err)` | 500, `{"error":"内部服务器错误"}` |
| H-12 | controlActionCN 已知 | `controlActionCN("start")` | `"开机"` |
| H-13 | controlActionCN 未知 | `controlActionCN("unknown")` | `"unknown"` (原样) |

### 6.2 AuthHandler (`auth.go`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| H-14 | Login 成功 | `svc.Login()` 返回 `(token, user, nil)` | 200, `{"token":"...","user":{...}}` |
| H-15 | Login 凭证错误 | `svc.Login()` 返回 `ErrInvalidCredentials` | 401 |
| H-16 | Login 内部错误 | `svc.Login()` 返回 `ErrInternal` | 500 |
| H-17 | Login 请求体解析失败 | 请求体 `{invalid json` | 400 |
| H-18 | Me 已认证 | 上下文含有效 Claims | 200, 含用户信息 |
| H-19 | Me 未认证 | 无 Claims | 401 |
| H-20 | Me 角色名查询失败 | 角色查询 DB 错误 | 200 (仅 Warn，仍返回数据) |

### 6.3 AdminHandler — ResetPassword (`admin.go`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| H-21 | 超级管理员无需旧密码 | `claims.RoleCode="super_admin"`, 请求含 `new_password` | 200 |
| H-22 | 普通用户需旧密码验证 | 请求含 `old_password` + `new_password`，`GetPasswordHash` 返回匹配哈希 | 200 |
| H-23 | 普通用户旧密码错误 | `bcrypt` 不匹配 | 403 |
| H-24 | 普通用户缺少旧密码 | 请求无 `old_password` | 400 |

### 6.4 AdminHandler — UpdateUser

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| H-25 | 部分更新 IsActive | 请求 `{"is_active":false}` | 200 |
| H-26 | 全零值字段被拒 | 所有字段为零值/空 | 400 ("至少需要一个字段") |
| H-27 | 自修改权限受限 | 用户修改自己的 role_id | 按 RBAC 规则 (非超管可能 403) |
| H-28 | 重复用户名 | `CreateUser` 返回 duplicate key error | 409 Conflict |

### 6.5 ProjectHandler (`project.go`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| H-29 | Get 正常 | `repo.GetByID` 返回 project | 200 |
| H-30 | Get 不存在 | `repo.GetByID` 返回 `(nil, nil)` | 404 |
| H-31 | List 空 | `repo.List` 返回空切片 | 200, `[]` |
| H-32 | SetProjectUsers 空列表 | 请求体 `[]` | 200 (仅事务性 DELETE) |

### 6.6 AlarmHandler (`alarm.go`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| H-33 | CreateRule 缺少名称 | 请求 `{"name":""}` | 400 |
| H-34 | CreateRule 默认 Enabled | 请求无 `enabled` 字段 | 写入 enabled=true |
| H-35 | ListLogs 含 today 过滤 | `?today=1` | WHERE 含 `DATE(alarm_at) = CURRENT_DATE` |
| H-36 | ListLogs 含 date 范围 | `?date_from=2026-01-01&date_to=2026-01-31` | WHERE 含范围条件 |
| H-37 | AckLog | `POST /alarm-logs/42/ack` | `ack_by` = 当前用户名, `ack_at` = NOW() |

### 6.7 TelemetryHandler (`telemetry.go`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| H-38 | Realtime 无 device_id | 无参数 | 全设备最新遥测 |
| H-39 | Realtime 指定设备 | `?device_id=5` | 仅 device 5 的最新值 |
| H-40 | History 缺少必填参数 | 无 `?metric=` | 400 或空结果 |
| H-41 | History 自定义 hours | `?device_id=5&metric=temp&hours=48` | 48 小时数据 |
| H-42 | Stats system 模式 | `?device_id=0` | 在线/离线计数 |

### 6.8 DashboardHandler (`dashboard.go`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| H-43 | ScreenData 超级管理员 | `claims.RoleCode="super_admin"` | 返回所有项目 |
| H-44 | ScreenData 普通用户已分配 | `claims.UserID=5`, project_user 含匹配行 | 仅返回分配的项目 |
| H-45 | ScreenData 普通用户未分配 | `claims.UserID=5`, project_user 无匹配 | 空项目列表 |
| H-46 | ScreenData 查询失败降级 | building 查询 DB 错误 | Warn 日志 + 空 buildings，不 500 |

### 6.9 LogHandler — Interval 注入安全 (`log.go`)

| ID | 场景 | 请求 | 预期 |
|---|---|---|---|
| H-47 | Interval 白名单分钟 | `?interval=minutes` | `date_trunc('minute', ...)` |
| H-48 | Interval 白名单小时 | `?interval=hour` | `date_trunc('hour', ...)` |
| H-49 | Interval 白名单天 | `?interval=day` | `date_trunc('day', ...)` |
| H-50 | Interval 白名单 raw | `?interval=raw` | 无聚合 |
| H-51 | Interval 非法值 | `?interval=DROP TABLE` | 被白名单拦截，回退或 400 |

### 6.10 StartupHandler (`startup.go`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| H-52 | Execute 创建执行 + 后台运行 | `POST /startup-plans/1/execute` | 201 + goroutine 运行，含 `defer recover()` |
| H-53 | StopExecution 运行中 | 状态 running → stopped | 200 |
| H-54 | StopExecution 已完成 | 状态 completed | 404 (RowsAffected=0) |
| H-55 | 定时任务 weekly 精确匹配 | `schedule_type=weekly, schedule_days="1,3,5"` | `regexp_split_to_array` 防止子串误匹配 |
| H-56 | 定时任务 once 仅执行一次 | `schedule_type=once, last_run_at IS NULL` | 成功后 `last_run_at` 非空 |

### 6.11 IntelligenceHandler (`intelligence.go`)

| ID | 场景 | 请求 | 预期 |
|---|---|---|---|
| H-57 | parseTimeParam RFC3339 | `"2026-06-26T12:00:00Z"` | 正确解析 |
| H-58 | parseTimeParam 无时区 | `"2026-06-26T12:00:00"` | 正确解析 (格式2) |
| H-59 | parseTimeParam 日期 | `"2026-06-26"` | 解析为当天 00:00 |
| H-60 | parseTimeParam 空 | `""` | 零值 `time.Time{}` |
| H-61 | PowerQuality 缺 device_id | 无 `?device_id=` | 400 或默认值 0 |
| H-62 | Forecast 天气失败降级 | weather 返回 error | Warn + outdoorTemp=0 |

### 6.12 DeviceHandler — Control (`models.go`)

| ID | 场景 | Mock | 预期 |
|---|---|---|---|
| H-63 | Control 正常 | `POST /devices/5/control {"action":"start"}` | control_record 写入 + 硬件分发 + 状态更新 |
| H-64 | Control 无 gwMgr | `h.gwMgr == nil` | 跳过硬件分发，仅写 control_record |
| H-65 | dispatchHardware 设备无配置 | 设备未关联网关 | 返回 error (跳过) |
| H-66 | dispatchHardware 网关离线 | `transport.IsConnected() == false` | 返回 error |
| H-67 | dispatchHardware deviceNo > 255 | `dev.DeviceNo=300` | 返回 error |
| H-68 | dispatchHardware 自定义协议节点地址 | `gatewayType="custom"` | 前置 `nodeAddr` 2 字节 |

---

## 7. 跨切面安全测试

| ID | 场景 | 验证点 |
|---|---|---|
| S-01 | JWT 认证全覆盖 | 所有 `/api/v1/*` 路径 (除 login/health) 无 Token → 401 |
| S-02 | RBAC 权限全覆盖 | 非公开端点缺少权限码 → 403 |
| S-03 | SQL 注入: interval | `interval=1; DROP TABLE users;--` → 白名单拦截 |
| S-04 | SQL 注入: 用户名 | `username=admin' OR '1'='1` → 参数化查询防护 |
| S-05 | 登录限流 | 同一 IP 60 秒内 11 次 POST /auth/login → 429 |
| S-06 | 密码日志脱敏 | DSN 日志不含明文密码 |
| S-07 | CORS 凭证安全 | 通配符 `*` + Authorization → 回显 Origin 非 `*` |
| S-08 | BodyLimit 防护 | >1MB 请求体 → MaxBytesReader 阻断 |
| S-09 | GetByID nil/nil 约定 | 所有 Handler.Get 方法: `err!=nil`→500, `obj==nil`→404 |
| S-10 | Goroutine panic 恢复 | 5 个后台 goroutine 均含 `defer recover()` |

---

## 8. 现有测试覆盖缺口汇总

### 8.1 紧急修复 (P0)

| 缺口 | 位置 | 问题 |
|---|---|---|
| **GetByID NotFound 测试不一致** | `repo_test.go` `TestBuildingRepoGetByIDNotFound` / `TestProjectRepoGetByIDNotFound` | 测试期望 error，但生产代码返回 `(nil, nil)`。测试会失败。 |
| **Auth Login/HasPermission 不可测** | `auth.go` | 使用具体 `*pgxpool.Pool` 而非 `DBTX`。需重构才能写单元测试。 |

### 8.2 高优先级 (P1)

| 缺口 | 范围 |
|---|---|
| 所有 13 个 Handler 文件的端点测试 | 零覆盖。仅辅助函数有测试 |
| DeviceRepo Create/Update/Delete | 未测试 |
| PropertyRepo GetByID/Update/Delete | 未测试 |
| RegisterRepo GetByID/Create/Delete | 未测试 |
| ProjectRepo SetProjectUsers 事务路径 | 未测试 |
| AdminRepo GetPasswordHash/DBStats | 未测试 |
| Alarm Evaluate/AlertOffline (DB 路径) | 未测试 |
| Orchestrator LoadPlan/StartExecution/Run/Finish | 全部未测试 (仅结构体字段测试) |
| Weather FetchWeather HTTP 调用路径 | 未测试 |

### 8.3 中优先级 (P2)

| 缺口 | 范围 |
|---|---|
| RequirePermission 中间件 | 未测试 |
| BodyLimit 中间件 | 未测试 |
| TransparentTransport SendAndReceive | 未测试 |
| CustomTransport SendAndReceive | 未测试 |
| Intelligence RunStrategies/RunFullAnalysis 编排 | 未测试 |
| 迁移 detectExistingVersion 完整链 | 未测试 |
| parseHex / applyMask 纯函数 | 未测试 |

---

## 9. 推荐的测试实施顺序

```
Phase 1 (本周): 修复 GetByID 测试不一致 + 补全 Repository 缺失方法
  → R-01, R-05~R-09, R-11~R-18, R-20~R-30
  
Phase 2: 纯函数 + 无依赖路径
  → A-01~A-09 (JWT+bcrypt), AL-01~AL-13 (triggered), I-01~I-05 (湿球),
    T-01~T-18 (decodeVal), G-01~G-22 (Modbus), CT-01~CT-11 (Custom帧)

Phase 3: Middleware + 安全
  → MW-01~MW-29, S-01~S-10

Phase 4: Service DB 集成路径
  → AL-14~AL-21 (告警), I-06~I-34 (智能), W-05~W-09 (天气缓存),
    M-01~M-09 (迁移)

Phase 5: Handler 端点
  → H-01~H-68 (覆盖所有处理器关键路径)

Phase 6: 重构 Auth Service 为 DBTX 接口 + 补充 Login/HasPermission 测试
```
