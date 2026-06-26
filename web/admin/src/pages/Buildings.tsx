import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Space, message, Popconfirm, Select } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useNavigate, useSearchParams } from 'react-router-dom';
import api from '../api/client';
import type { Building, Device } from '../types';

export default function Buildings() {
  const [data, setData] = useState<Building[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Building | null>(null);
  const [projects, setProjects] = useState<any[]>([]);
  const [allDevices, setAllDevices] = useState<Device[]>([]);
  const [filterProject, setFilterProject] = useState<number | null>(null);
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const fetch = async () => {
    setLoading(true);
    try {
      const [b, p, d] = await Promise.all([
        api.get('/buildings'),
        api.get('/projects'),
        api.get('/devices'),
      ]);
      setData(b.data); setProjects(p.data); setAllDevices(d.data);
    } catch {
      message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { fetch(); }, []);

  // Read project_id from URL on mount
  useEffect(() => {
    const pid = searchParams.get('project_id');
    if (pid) setFilterProject(Number(pid));
  }, []);

  // Sync filterProject back to URL
  useEffect(() => {
    if (filterProject) {
      setSearchParams({ project_id: String(filterProject) });
    } else {
      setSearchParams({});
    }
  }, [filterProject]);

  const deviceCountByBuilding = (buildingId: number) =>
    allDevices.filter((d: Device) => Number(d.building_id) === buildingId).length;

  const filteredData = filterProject
    ? data.filter(b => Number(b.project_id) === filterProject)
    : data;

  const save = async (v: any) => {
    try {
      if (editing) { await api.put('/buildings/' + editing.id, v); message.success('更新成功'); }
      else { await api.post('/buildings', v); message.success('创建成功'); }
      setModalOpen(false); setEditing(null); form.resetFields(); fetch();
    } catch { message.error('保存失败'); }
  };
  const del = async (id: number) => {
    try { await api.delete('/buildings/' + id); message.success('已删除'); fetch(); }
    catch { message.error('删除失败'); }
  };

  const cols = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '名称', dataIndex: 'name' },
    { title: '所属项目', dataIndex: 'project_id', render: (v: number) => projects.find((p: any) => p.id === v)?.name || v },
    {
      title: '设备数', width: 80, render: (_: any, r: Building) => {
        const n = deviceCountByBuilding(r.id);
        return n > 0
          ? <a onClick={() => navigate(`/devices?building_id=${r.id}`)} style={{ fontWeight: 500 }}>{n}</a>
          : <span style={{ color: '#999' }}>0</span>;
      }
    },
    { title: '节能率', dataIndex: 'save_rate', render: (v: number) => v ? (v * 100).toFixed(1) + '%' : '-' },
    { title: '创建时间', dataIndex: 'created_at', render: (v: string) => v?.slice(0, 10) },
    {
      title: '操作', render: (_: any, r: Building) => (
        <Space size="small">
          <a onClick={() => { setEditing(r); form.setFieldsValue({ ...r, save_rate: r.save_rate ? (r.save_rate * 100).toFixed(1) : '', outdoor_temp: r.outdoor_temp, outdoor_humidity: r.outdoor_humidity }); setModalOpen(true); }}>编辑</a>
          <a onClick={() => navigate(`/startup-plans?building_id=${r.id}`)}>启停</a>
          <Popconfirm title="确定删除?" onConfirm={() => del(r.id)}><a style={{ color: 'red' }}>删除</a></Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>楼宇管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setModalOpen(true); }}>新增</Button>
      </div>
      <div style={{ display: 'flex', gap: 16, marginBottom: 16, alignItems: 'flex-end' }}>
        <div>
          <div style={{ marginBottom: 4, color: '#666', fontSize: 12 }}>筛选项目</div>
          <Select style={{ width: 220 }} placeholder="全部项目" allowClear
            value={filterProject}
            onChange={(v) => setFilterProject(v ? Number(v) : null)}
            options={projects.map((p: any) => ({ value: p.id, label: p.name }))} />
        </div>
        {filterProject && <div style={{ paddingBottom: 4, color: '#006875', fontWeight: 500 }}>{filteredData.length} 个楼宇</div>}
      </div>
      <Table rowKey="id" columns={cols} dataSource={filteredData} loading={loading} />
      <Modal title={editing ? '编辑' : '新增'} open={modalOpen} onOk={form.submit} onCancel={() => setModalOpen(false)}>
        <Form form={form} layout="vertical" onFinish={(v: any) => save({ ...v, save_rate: v.save_rate ? parseFloat(v.save_rate) / 100 : 0 })}>
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
