import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Space, message, Popconfirm, Select } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import api from '../api/client';

interface Building { id: number; project_id: number; name: string; save_rate: number; created_at: string }

export default function Buildings() {
  const [data, setData] = useState<Building[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Building | null>(null);
  const [projects, setProjects] = useState<any[]>([]);
  const [form] = Form.useForm();

  const fetch = async () => {
    setLoading(true);
    const [b, p] = await Promise.all([api.get('/buildings'), api.get('/projects')]);
    setData(b.data); setProjects(p.data); setLoading(false);
  };
  useEffect(() => { fetch(); }, []);

  const save = async (v: any) => {
    if (editing) { await api.put('/buildings/'+editing.id, v); message.success('更新成功'); }
    else { await api.post('/buildings', v); message.success('创建成功'); }
    setModalOpen(false); setEditing(null); form.resetFields(); fetch();
  };
  const del = async (id: number) => { await api.delete('/buildings/'+id); message.success('已删除'); fetch(); };

  const cols = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '名称', dataIndex: 'name' },
    { title: '所属项目', dataIndex: 'project_id', render: (v: number) => projects.find((p: any) => p.id === v)?.name || v },
    { title: '节能率', dataIndex: 'save_rate', render: (v: number) => v ? (v*100).toFixed(1)+'%' : '-' },
    { title: '创建时间', dataIndex: 'created_at', render: (v: string) => v?.slice(0, 10) },
    { title: '操作', render: (_: any, r: Building) => (
      <Space><a onClick={() => { setEditing(r); form.setFieldsValue({...r, save_rate: r.save_rate ? (r.save_rate*100).toFixed(1) : ''}); setModalOpen(true); }}>编辑</a>
      <Popconfirm title="确定删除?" onConfirm={() => del(r.id)}><a style={{ color: 'red' }}>删除</a></Popconfirm></Space>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>楼宇管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setModalOpen(true); }}>新增</Button>
      </div>
      <Table rowKey="id" columns={cols} dataSource={data} loading={loading} />
      <Modal title={editing ? '编辑' : '新增'} open={modalOpen} onOk={form.submit} onCancel={() => setModalOpen(false)}>
        <Form form={form} layout="vertical" onFinish={(v: any) => save({...v, save_rate: v.save_rate ? parseFloat(v.save_rate)/100 : 0})}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="project_id" label="所属项目" rules={[{ required: true }]}>
            <Select options={projects.map((p: any) => ({ value: p.id, label: p.name }))} />
          </Form.Item>
          <Form.Item name="save_rate" label="节能率(%)"><InputNumber style={{ width: '100%' }} min={0} max={100} placeholder="如: 25" /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
