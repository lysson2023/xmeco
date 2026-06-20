import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Space, message, Popconfirm, Select } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import api from '../api/client';

interface Project { id: number; name: string; agent_id: number | null; address: string; admin_code: string; created_at: string }

export default function Projects() {
  const [data, setData] = useState<Project[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Project | null>(null);
  const [agents, setAgents] = useState<any[]>([]);
  const [form] = Form.useForm();

  const fetch = async () => {
    setLoading(true);
    const [p, a] = await Promise.all([api.get('/projects'), api.get('/agents')]);
    setData(p.data); setAgents(a.data); setLoading(false);
  };
  useEffect(() => { fetch(); }, []);

  const save = async (values: any) => {
    if (editing) {
      await api.put('/projects/'+editing.id, values);
      message.success('修改成功');
    } else {
      await api.post('/projects', values);
      message.success('创建成功');
    }
    setModalOpen(false); setEditing(null); form.resetFields(); fetch();
  };

  const del = async (id: number) => { await api.delete('/projects/'+id); message.success('已删除'); fetch(); };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '项目名称', dataIndex: 'name' },
    { title: '代理商', dataIndex: 'agent_id', render: (v: number|null) => agents.find(a => a.id===v)?.name || '-' },
    { title: '地址', dataIndex: 'address' },
    { title: '行政区划代码', dataIndex: 'admin_code', render: (v: string) => v || '-' },
    { title: '创建时间', dataIndex: 'created_at', render: (v: string) => v?.slice(0, 10) },
    { title: '操作', render: (_: any, r: Project) => (
      <Space>
        <a onClick={() => { setEditing(r); form.setFieldsValue(r); setModalOpen(true); }}>编辑</a>
        <Popconfirm title="确定删除?" onConfirm={() => del(r.id)}><a style={{ color: 'red' }}>删除</a></Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>项目管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setModalOpen(true); }}>新建项目</Button>
      </div>
      <Table rowKey="id" columns={columns} dataSource={data} loading={loading} />
      <Modal title={editing ? '编辑' : '新建'} open={modalOpen} onOk={form.submit} onCancel={() => setModalOpen(false)}>
        <Form form={form} layout="vertical" onFinish={save}>
          <Form.Item name="name" label="项目名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="agent_id" label="代理商"><Select allowClear placeholder="选择代理商" options={agents.map((a: any) => ({ value: a.id, label: a.name }))} /></Form.Item>
          <Form.Item name="address" label="地址"><Input /></Form.Item>
          <Form.Item name="admin_code" label="行政区划代码"><Input /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
