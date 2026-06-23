-- XMECO Seed Data V002
-- 角色 + 权限点种子数据

-- ============================================
-- 角色定义
-- ============================================
INSERT INTO role (code, name, level, is_system) VALUES
('super_admin',     '超级管理员',   0,  true),
('admin',           '平台管理员',   10, true),
('agent_admin',     '代理商管理员', 20, true),
('agent_operator',  '代理商运维',   30, true),
('agent_viewer',    '代理商查看者', 40, true),
('project_admin',   '项目管理员',   50, true),
('project_operator','项目运维',     60, true),
('project_viewer',  '项目查看者',   70, true)
ON CONFLICT (code) DO NOTHING;

-- ============================================
-- 权限点定义
-- ============================================
INSERT INTO permission (code, name, perm_group) VALUES
('project.view',        '查看项目',      'project'),
('project.create',      '创建项目',      'project'),
('project.edit',        '编辑项目',      'project'),
('project.delete',      '删除项目',      'project'),
('building.view',       '查看楼宇',      'building'),
('building.create',     '创建楼宇',      'building'),
('building.edit',       '编辑楼宇',      'building'),
('building.delete',     '删除楼宇',      'building'),
('device.view',         '查看设备',      'device'),
('device.create',       '添加设备',      'device'),
('device.edit',         '编辑设备',      'device'),
('device.delete',       '删除设备',      'device'),
('device.control',      '控制设备',      'device'),
('device.property',     '管理属性',      'device'),
('device.register',     '管理寄存器',    'device'),
('monitor.realtime',    '实时监控',      'monitor'),
('monitor.graph',       '查看图表',      'monitor'),
('monitor.alarm_config','告警配置',      'monitor'),
('monitor.control_log', '控制记录',      'monitor'),
('schedule.view',       '查看定时任务',  'schedule'),
('schedule.create',     '创建定时任务',  'schedule'),
('schedule.edit',       '编辑定时任务',  'schedule'),
('schedule.delete',     '删除定时任务',  'schedule'),
('user.view',           '查看用户',      'user'),
('user.create',         '创建用户',      'user'),
('user.edit',           '编辑用户',      'user'),
('user.delete',         '删除用户',      'user'),
('user.assign_role',    '分配角色',      'user'),
('agent.view',          '查看代理商',    'agent'),
('agent.create',        '创建代理商',    'agent'),
('agent.edit',          '编辑代理商',    'agent'),
('agent.delete',        '删除代理商',    'agent'),
('report.export',       '导出报表',      'report'),
('report.excel',        '导出Excel',     'report'),
('system.config',       '系统配置',      'system'),
('system.gateway',      '网关管理',      'system'),
('system.db',           '数据库管理',    'system')
ON CONFLICT (code) DO NOTHING;

-- ============================================
-- 默认权限绑定：super_admin 拥有全部权限
-- ============================================
INSERT INTO role_permission (role_id, perm_id)
SELECT r.id, p.id
FROM role r, permission p
WHERE r.code = 'super_admin'
ON CONFLICT (role_id, perm_id) DO NOTHING;

-- ============================================
-- 默认权限绑定：agent_admin（代理商管理员）
-- ============================================
INSERT INTO role_permission (role_id, perm_id)
SELECT r.id, p.id
FROM role r, permission p
WHERE r.code = 'agent_admin'
  AND (
    p.perm_group IN ('project','building','device','monitor','schedule','report')
    OR p.code IN ('user.view','user.create','user.edit')
  )
ON CONFLICT (role_id, perm_id) DO NOTHING;

-- ============================================
-- 默认权限绑定：agent_operator（代理商运维）
-- ============================================
INSERT INTO role_permission (role_id, perm_id)
SELECT r.id, p.id
FROM role r, permission p
WHERE r.code = 'agent_operator'
  AND p.code IN (
    'project.view','building.view',
    'device.view','device.create','device.edit','device.control',
    'device.property','device.register',
    'monitor.realtime','monitor.graph','monitor.alarm_config','monitor.control_log',
    'schedule.view','schedule.create','schedule.edit',
    'user.view','report.export','report.excel'
  )
ON CONFLICT (role_id, perm_id) DO NOTHING;

-- ============================================
-- 默认权限绑定：agent_viewer（纯查看）
-- ============================================
INSERT INTO role_permission (role_id, perm_id)
SELECT r.id, p.id
FROM role r, permission p
WHERE r.code = 'agent_viewer'
  AND p.code IN (
    'project.view','building.view','device.view',
    'monitor.realtime','monitor.graph','report.export'
  )
ON CONFLICT (role_id, perm_id) DO NOTHING;

-- ============================================
-- 创建默认超级管理员（密码: admin123，部署后立即修改）
-- ============================================
INSERT INTO users (username, password_hash, role_id, is_active)
SELECT 'admin', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
       r.id, true
FROM role r WHERE r.code = 'super_admin'
ON CONFLICT (username) DO NOTHING;

-- admin (platform admin): all except system-level and agent management
INSERT INTO role_permission (role_id, perm_id)
SELECT r.id, p.id FROM role r, permission p
WHERE r.code = 'admin'
  AND p.code NOT IN ('system.config','system.gateway','system.db','agent.create','agent.edit','agent.delete','user.assign_role')
ON CONFLICT (role_id, perm_id) DO NOTHING;

-- project_admin: full CRUD within their scope
INSERT INTO role_permission (role_id, perm_id)
SELECT r.id, p.id FROM role r, permission p
WHERE r.code = 'project_admin'
  AND p.perm_group IN ('project','building','device','monitor','schedule','report')
ON CONFLICT (role_id, perm_id) DO NOTHING;

-- project_operator: operate devices + monitor
INSERT INTO role_permission (role_id, perm_id)
SELECT r.id, p.id FROM role r, permission p
WHERE r.code = 'project_operator'
  AND p.code IN (
    'project.view','building.view',
    'device.view','device.create','device.edit','device.control','device.property','device.register',
    'monitor.realtime','monitor.graph','monitor.alarm_config','monitor.control_log',
    'schedule.view','schedule.create','schedule.edit',
    'report.export','report.excel'
  )
ON CONFLICT (role_id, perm_id) DO NOTHING;

-- project_viewer: view only
INSERT INTO role_permission (role_id, perm_id)
SELECT r.id, p.id FROM role r, permission p
WHERE r.code = 'project_viewer'
  AND p.code IN (
    'project.view','building.view','device.view',
    'monitor.realtime','monitor.graph',
    'report.export'
  )
ON CONFLICT (role_id, perm_id) DO NOTHING;

-- 修复历史权限误绑：清除 agent_viewer 错误获得的用户管理权限
DELETE FROM role_permission rp
USING role r, permission p
WHERE rp.role_id = r.id AND rp.perm_id = p.id
  AND r.code = 'agent_viewer'
  AND p.code IN ('user.view','user.create','user.edit');

DELETE FROM role_permission rp
USING role r, permission p
WHERE rp.role_id = r.id AND rp.perm_id = p.id
  AND r.code IN ('project_admin','project_operator','project_viewer')
  AND p.code IN ('user.view','user.create','user.edit');

DELETE FROM role_permission rp
USING role r, permission p
WHERE rp.role_id = r.id AND rp.perm_id = p.id
  AND r.code = 'agent_operator'
  AND p.code IN ('user.create','user.edit');
