# Bug 修复记录：大屏项目权限与显示问题

> 日期：2026-06-29  
> 影响模块：后端 `dashboard.go` ScreenData 接口  
> 影响范围：Web 大屏（:3001）项目选择器

---

## 问题描述

### 现象 1：权限不一致
- 后台项目管理中，"前海港湾酒店"只分配给了 `admin`，未分配给 `test`
- 但 Web 大屏上用 `test` 登录，仍能看到"前海港湾酒店"

### 现象 2：项目下拉框显示数字而非名称
- Web 大屏 `test` 登录后，项目下拉框显示 "14"（项目 ID），而非 "南山商场"

### 现象 3：登录后未自动选中默认项目
- 每个用户设有 `default_project_id`（admin→前海港湾酒店，test→南山商场）
- 登录大屏时应自动展示默认项目的数据，但实际未自动选中
- 原因同样是 Bug 2 导致 `projects` 返回空数组，前端自动选中逻辑的触发条件 `data.projects?.length` 不满足

---

## 根因分析

### Bug 1：`super_admin` / `admin` 绕过项目权限控制

**文件**：`internal/api/handler/dashboard.go` 第 102-106 行

```go
// 旧代码
platformAdmin = claims != nil && (claims.RoleCode == auth.RoleSuperAdmin || claims.RoleCode == auth.RoleAdmin)
if platformAdmin {
    projQ = `SELECT id,name FROM project ORDER BY id`  // ← 直接返回所有项目
}
```

`super_admin` 和 `admin` 角色走特权分支，无视 `project_user` 表的分配关系，直接返回全部项目。而 `test` 用户角色是 `super_admin`，因此大屏上能看到所有项目。

### Bug 2：UNION 查询 ORDER BY 语法错误（连带暴露）

**文件**：`internal/api/handler/dashboard.go` 第 103-106 行（旧代码）

```sql
-- 旧 SQL：PostgreSQL 不允许 UNION 后直接用表达式 ORDER BY
SELECT id,name FROM project WHERE id=$1
 UNION
 SELECT p.id,p.name FROM project p JOIN project_user pu ...
 ORDER BY CASE WHEN id=$1 THEN 0 ELSE 1 END, id   -- ← 非法！
```

**PostgreSQL 报错**：
```
ERROR: invalid UNION/INTERSECT/EXCEPT ORDER BY clause
DETAIL: Only result column names can be used, not expressions or functions.
```

这个 bug 一直存在，但因为 `super_admin` 走了 Bug 1 的特权分支从未触发。修复 Bug 1 后，所有用户走 UNION 查询，SQL 报错 → `projects` 返回空数组 → 前端 Ant Design Select 找不到匹配的 label，fallback 显示原始 `value`（即项目 ID 数字 14）。

---

## 修复方案

### 修复 1：统一项目权限逻辑

移除 `platformAdmin` 特权分支，所有角色统一走 `default_project_id + project_user` 的权限控制：

```go
// 新代码：所有角色统一逻辑
if defaultPID > 0 {
    // UNION：默认项目 + project_user 授权的项目
    projQ = `SELECT * FROM (
         SELECT id,name FROM project WHERE id=$1
         UNION
         SELECT p.id,p.name FROM project p JOIN project_user pu ON pu.project_id=p.id WHERE pu.user_id=$2
    ) sub ORDER BY CASE WHEN id=$1 THEN 0 ELSE 1 END, id`
} else {
    // 无默认项目 → 只看 project_user
    projQ = `SELECT p.id,p.name FROM project p JOIN project_user pu ON pu.project_id=p.id WHERE pu.user_id=$1 ORDER BY p.id`
}
```

### 修复 2：UNION ORDER BY 语法修正

将 UNION 查询包裹在子查询中：

```sql
-- 新 SQL：子查询包裹，ORDER BY 表达式合法
SELECT * FROM (
    SELECT id,name FROM project WHERE id=$1
    UNION
    SELECT p.id,p.name FROM project p JOIN project_user pu ...
) sub ORDER BY CASE WHEN id=$1 THEN 0 ELSE 1 END, id
```

---

## 修复后行为

### 项目可见性

| 用户 | 角色 | 大屏可见项目 |
|---|---|---|
| admin | super_admin | 前海港湾酒店、南山商场（均为 project_user 分配） |
| test | super_admin | 仅南山商场（仅分配了项目 14） |

- 项目下拉框正确显示**项目名称**
- 与后台项目管理的分配关系**完全一致**

### 默认项目自动选中

前端 `Screen.tsx` 第 127-136 行已有自动选中逻辑：

```typescript
useEffect(() => {
    if (!pid && data.projects?.length) {
        const defaultId = data.default_project_id;
        if (defaultId && data.projects.some(p => p.id === defaultId)) {
            setPid(defaultId);   // 优先选默认项目
        } else {
            setPid(data.projects[0].id);
        }
    }
}, [data.projects, pid, data.default_project_id]);
```

修复后流程正常：登录 → `pid=0` → API 返回 `default_project_id` + `projects` → 自动 `setPid(默认项目)` → 触发加载该项目数据。

| 用户 | 默认项目 | 登录后自动显示 |
|---|---|---|
| admin | 前海港湾酒店 (id=7) | ✅ 自动选中，下拉可切换到南山商场 |
| test | 南山商场 (id=14) | ✅ 自动选中 |

---

## 涉及文件

| 文件 | 变更 |
|---|---|
| `internal/api/handler/dashboard.go` | 移除 platformAdmin 特权 + 修复 UNION SQL 子查询 |
| `internal/api/handler/dashboard_test.go` | 重写 H-43~H-46 测试用例匹配新逻辑 |

---

## 验证

```bash
# 编译
go build -o server.exe ./cmd/server

# 测试
go test ./internal/api/handler/ -run TestDashboardHandler -v
# --- PASS: TestDashboardHandler_ScreenData (5 sub-tests)
# --- PASS: TestDashboardHandler_ScreenData_BuildingQueryFail
# --- PASS: TestDashboardHandler_GetConfig

# API 验证
curl -H "Authorization: Bearer <token>" http://localhost:9090/api/v1/screen/data
# admin → projects: [前海港湾酒店, 南山商场]
# test  → projects: [南山商场]
```
