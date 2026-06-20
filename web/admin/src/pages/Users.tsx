import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Select, Space, message, Popconfirm, Switch } from 'antd';
import { PlusOutlined, KeyOutlined } from '@ant-design/icons';
import api from '../api/client';

export default function Users() {
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [pwdOpen, setPwdOpen] = useState(false);
  const [pwdUserId, setPwdUserId] = useState<number | null>(null);
  const [roles, setRoles] = useState<any[]>([]);
  const [agents, setAgents] = useState<any[]>([]);
  const [form] = Form.useForm();
  const [pwdForm] = Form.useForm();

  const fetch = async () => {
    setLoading(true);
    const [u, r, a] = await Promise.all([api.get('/users'), api.get('/roles'), api.get('/agents')]);
    setData(u.data); setRoles(r.data); setAgents(a.data); setLoading(false);
  };
  useEffect(() => { fetch(); }, []);

  const save = async (v: any) => {
    await api.post('/users', v);
    message.success('创建成功');
    setModalOpen(false); form.resetFields(); fetch();
  };

  const toggleActive = async (id: number, checked: boolean) => {
    const user = data.find(u => u.id === id);
    if (!user) return;
    await api.put('/users/'+id, { role_id: user.role_id, agent_id: user.agent_id, is_active: checked });
    message.success(checked ? '已启用' : '已禁用');
    fetch();
  };

  const resetPwd = async (v: { password: string }) => {
    await api.post('/users/'+pwdUserId+'/reset-password', v);
    message.success('密码已重置');
    setPwdOpen(false); pwdForm.resetFields();
  };

  const del = async (id: number) => { await api.delete('/users/'+id); message.success('已删除'); fetch(); };

  const cols = [
    { title: 'ID', dataIndex: 'id', width: 50 },
    { title: '用户名', dataIndex: 'username' },
    { title: '角色', dataIndex: 'role_name' },
    { title: '代理商', dataIndex: 'agent_name', render: (v: string | null) => v || '-' },
    { title: '备注', dataIndex: 'remark', render: (v: string | null) => v || '-' },
    { title: '状态', dataIndex: 'is_active', render: (v: boolean, r: any) =>
      <Switch checked={v} onChange={(c: boolean) => toggleActive(r.id, c)} checkedChildren="启用" unCheckedChildren="禁用" />
    },
    { title: '最后登录', dataIndex: 'last_login_at', render: (v: string | null) => v?.slice(0, 19) || '-' },
    { title: '创建时间', dataIndex: 'created_at', render: (v: string) => v?.slice(0, 10) },
    { title: '操作', render: (_: any, r: any) => (
      <Space>
        <a onClick={() => { setPwdUserId(r.id); pwdForm.resetFields(); setPwdOpen(true); }}><KeyOutlined /> 重置密码</a>
        <Popconfirm title="确定删除?" onConfirm={() => del(r.id)}><a style={{ color: 'red' }}>删除</a></Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>用户管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setModalOpen(true); }}>新增用户</Button>
      </div>
      <Table rowKey="id" columns={cols} dataSource={data} loading={loading} />

      <Modal title="新增用户" open={modalOpen} onOk={form.submit} onCancel={() => setModalOpen(false)}>
        <Form form={form} layout="vertical" onFinish={save}>
          <Form.Item name="username" label="用户名" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true }]}><Input.Password /></Form.Item>
          <Form.Item name="role_id" label="角色" rules={[{ required: true }]}>
            <Select options={roles.map((r: any) => ({ value: r.id, label: r.name }))} />
          </Form.Item>
          <Form.Item name="agent_id" label="代理商">
            <Select allowClear options={agents.map((a: any) => ({ value: a.id, label: a.name }))} />
          </Form.Item>
          <Form.Item name="remark" label="备注"><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>

      <Modal title="重置密码" open={pwdOpen} onOk={pwdForm.submit} onCancel={() => setPwdOpen(false)}>
        <Form form={pwdForm} layout="vertical" onFinish={resetPwd}>
          <Form.Item name="password" label="新密码" rules={[{ required: true }]}><Input.Password /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}