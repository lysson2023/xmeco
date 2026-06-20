import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Space, message, Popconfirm } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import api from '../api/client';

interface Agent { id: number; name: string; created_at: string }

export default function Agents() {
  const [data, setData] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Agent | null>(null);
  const [form] = Form.useForm();

  const fetch = async () => {
    setLoading(true);
    const res = await api.get('/agents');
    setData(res.data);
    setLoading(false);
  };
  useEffect(() => { fetch(); }, []);

  const save = async (v: any) => {
    if (editing) {
      await api.put('/agents/'+editing.id, v);
      message.success('更新成功');
    } else {
      await api.post('/agents', v);
      message.success('创建成功');
    }
    setModalOpen(false); setEditing(null); form.resetFields(); fetch();
  };

  const del = async (id: number) => {
    await api.delete('/agents/'+id);
    message.success('已删除');
    fetch();
  };

  const cols = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '名称', dataIndex: 'name' },
    { title: '创建时间', dataIndex: 'created_at', render: (v: string) => v?.slice(0, 10) },
    { title: '操作', render: (_: any, r: Agent) => (
      <Space>
        <a onClick={() => { setEditing(r); form.setFieldsValue(r); setModalOpen(true); }}>编辑</a>
        <Popconfirm title="确定删除?" onConfirm={() => del(r.id)}>
          <a style={{ color: 'red' }}>删除</a>
        </Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>代理商管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setModalOpen(true); }}>新增代理商</Button>
      </div>
      <Table rowKey="id" columns={cols} dataSource={data} loading={loading} />
      <Modal title={editing ? '编辑' : '新增'} open={modalOpen} onOk={form.submit} onCancel={() => setModalOpen(false)}>
        <Form form={form} layout="vertical" onFinish={save}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}><Input /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
