import { useEffect, useState } from 'react';
import { Card, Checkbox, Button, message, Tag, Space } from 'antd';
import { SaveOutlined } from '@ant-design/icons';
import api from '../api/client';

interface Role { id: number; code: string; name: string; level: number; is_system: boolean }
interface Permission { id: number; code: string; name: string; perm_group: string }

const groupNames: Record<string, string> = {
  project: '项目管理', building: '建筑管理', device: '设备管理',
  monitor: '监控管理', schedule: '定时任务', user: '用户管理',
  agent: '代理商管理', report: '报表管理', system: '系统管理',
};

export default function Permissions() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [selectedRole, setSelectedRole] = useState<number | null>(null);
  const [checkedPerms, setCheckedPerms] = useState<number[]>([]);
  const [loading, setLoading] = useState(false);

  const fetch = async () => {
    setLoading(true);
    try {
      const [r, p] = await Promise.all([api.get('/roles'), api.get('/permissions')]);
      setRoles(r.data); setPermissions(p.data);
      if (r.data.length > 0 && !selectedRole) {
        setSelectedRole(r.data[0].id);
      }
    } catch {
      message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { fetch(); }, []);

  useEffect(() => {
    if (selectedRole) {
      api.get('/roles/'+selectedRole+'/permissions').then(res => {
        setCheckedPerms(res.data.perm_ids || []);
      });
    }
  }, [selectedRole]);

  const saveRolePerms = async () => {
    if (!selectedRole) return;
    try {
      await api.put('/roles/'+selectedRole+'/permissions', { perm_ids: checkedPerms });
      message.success('权限已保存');
    } catch {
      message.error('保存失败');
    }
  };

  const groupedPerms: Record<string, Permission[]> = {};
  permissions.forEach(p => {
    if (!groupedPerms[p.perm_group]) groupedPerms[p.perm_group] = [];
    groupedPerms[p.perm_group].push(p);
  });

  const allPermIds = permissions.map(p => p.id);
  const checkAll = (checked: boolean) => {
    setCheckedPerms(checked ? allPermIds : []);
  };

  const rolePermCount: Record<number, number> = {};
  // We compute on the fly based on checkedPerms
  const roleColumns = [
    { title: '角色名称', dataIndex: 'name' },
    { title: '角色编码', dataIndex: 'code' },
    { title: '级别', dataIndex: 'level' },
    { title: '类型', dataIndex: 'is_system', render: (v: boolean) => v ? <Tag color="blue">系统</Tag> : <Tag>自定义</Tag> },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>权限管理</h2>
      <div style={{ display: 'flex', gap: 24 }}>
        <Card title="角色列表" size="small" style={{ width: 280 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
            {roles.map(r => (
              <Button
                key={r.id}
                type={selectedRole === r.id ? 'primary' : 'default'}
                size="small"
                onClick={() => setSelectedRole(r.id)}
                style={{ textAlign: 'left' }}
              >
                {r.name}
              </Button>
            ))}
          </div>
        </Card>

        <Card
          title={selectedRole ? '编辑权限 - ' + roles.find(r => r.id === selectedRole)?.name : '请选择角色'}
          size="small"
          style={{ flex: 1 }}
          extra={
            <Space>
              <Button size="small" onClick={() => checkAll(true)}>全选</Button>
              <Button size="small" onClick={() => checkAll(false)}>全不选</Button>
              <Button type="primary" size="small" icon={<SaveOutlined />} onClick={saveRolePerms}>保存</Button>
            </Space>
          }
        >
          {Object.entries(groupedPerms).map(([group, perms]) => (
            <div key={group} style={{ marginBottom: 16 }}>
              <div style={{ fontWeight: 600, marginBottom: 8, color: '#006875' }}>
                {groupNames[group] || group}
              </div>
              <Checkbox.Group
                value={checkedPerms}
                onChange={(vals: any[]) => setCheckedPerms(vals)}
                style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 16px' }}
              >
                {perms.map(p => (
                  <Checkbox key={p.id} value={p.id}>{p.name}</Checkbox>
                ))}
              </Checkbox.Group>
            </div>
          ))}
          {!selectedRole && <div style={{ color: '#999', padding: 40, textAlign: 'center' }}>请从左侧选择一个角色</div>}
        </Card>
      </div>
    </div>
  );
}
