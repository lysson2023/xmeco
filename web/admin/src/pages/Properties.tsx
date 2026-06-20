import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Select, Space, message, Popconfirm, Switch } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import api from '../api/client';

const OP_TYPES = ['只读', '数值', '模式选择', '开关机'];

export default function Properties() {
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [projects, setProjects] = useState<any[]>([]);
  const [allBuildings, setAllBuildings] = useState<any[]>([]);
  const [buildings, setBuildings] = useState<any[]>([]);
  const [devices, setDevices] = useState<any[]>([]);
  const [selectedProject, setSelectedProject] = useState<number | null>(null);
  const [selectedBuilding, setSelectedBuilding] = useState<number | null>(null);
  const [selectedDevice, setSelectedDevice] = useState<number | null>(null);
  const [form] = Form.useForm();

  useEffect(() => { api.get('/projects').then(r => setProjects(r.data)); }, []);
  useEffect(() => { api.get('/buildings').then(r => setAllBuildings(r.data)); }, []);

  useEffect(() => {
    if (selectedProject) {
      setBuildings(allBuildings.filter((b: any) => Number(b.project_id) === Number(selectedProject)));
      setSelectedBuilding(null);
      setSelectedDevice(null);
    } else { setBuildings([]); setSelectedBuilding(null); setSelectedDevice(null); }
  }, [selectedProject, allBuildings]);

  useEffect(() => {
    if (selectedBuilding) {
      api.get('/devices?building_id='+selectedBuilding).then(r => setDevices(r.data));
      setSelectedDevice(null);
    } else { setDevices([]); setSelectedDevice(null); }
  }, [selectedBuilding]);

  useEffect(() => {
    if (selectedDevice) {
      setLoading(true);
      api.get('/properties?device_id='+selectedDevice).then(r => { setData(r.data); setLoading(false); });
    } else { setData([]); }
  }, [selectedDevice]);

  const save = async (v: any) => {
    const payload = { ...v, device_id: selectedDevice, is_key: v.is_key || false };
    if (editing) { await api.put('/properties/'+editing.id, payload); message.success('保存成功'); }
    else { await api.post('/properties', payload); message.success('保存成功'); }
    setModalOpen(false); setEditing(null); form.resetFields();
    if (selectedDevice) api.get('/properties?device_id='+selectedDevice).then(r => setData(r.data));
  };
  const del = async (id: number) => {
    await api.delete('/properties/'+id); message.success('已删除');
    if (selectedDevice) api.get('/properties?device_id='+selectedDevice).then(r => setData(r.data));
  };

  const cols = [
    { title: 'ID', dataIndex: 'id', width: 45 },
    { title: '属性名', dataIndex: 'prop_name', width: 120 },
    { title: '属性值', dataIndex: 'prop_value', width: 90, render: (v: string) => v || '-' },
    { title: '单位', dataIndex: 'unit', width: 60, render: (v: string) => v || '-' },
    { title: '操作类型', dataIndex: 'operation_type', width: 90 },
    { title: '最小值', dataIndex: 'min_value', width: 70, render: (v: string) => v || '-' },
    { title: '最大值', dataIndex: 'max_value', width: 70, render: (v: string) => v || '-' },
    { title: '关键属性', dataIndex: 'is_key', width: 80, render: (v: boolean) => v ? '是' : '否' },
    { title: '简称', dataIndex: 'prop_short', width: 80, render: (v: string) => v || '-' },
    { title: '操作', width: 100, render: (_: any, r: any) => (
      <Space>
        <a onClick={() => { setEditing(r); form.setFieldsValue(r); setModalOpen(true); }}>编辑</a>
        <Popconfirm title="确定删除?" onConfirm={() => del(r.id)}><a style={{ color: 'red' }}>删除</a></Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>属性配置</h2>
        <Button type="primary" icon={<PlusOutlined />} disabled={!selectedDevice}
          onClick={() => { setEditing(null); form.resetFields(); setModalOpen(true); }}>新增</Button>
      </div>
      <div style={{ display: 'flex', gap: 16, marginBottom: 16, alignItems: 'flex-end' }}>
        <div>
          <div style={{ marginBottom: 4, color: '#666', fontSize: 12 }}>选择项目</div>
          <Select style={{ width: 200 }} placeholder="请选择项目" allowClear value={selectedProject}
            onChange={(v) => setSelectedProject(v ? Number(v) : null)}
            options={projects.map((p: any) => ({ value: p.id, label: p.name }))} />
        </div>
        <div>
          <div style={{ marginBottom: 4, color: '#666', fontSize: 12 }}>选择楼宇</div>
          <Select style={{ width: 220 }} placeholder="请选择楼宇" allowClear value={selectedBuilding}
            disabled={!selectedProject} onChange={(v) => setSelectedBuilding(v ? Number(v) : null)}
            options={buildings.map((b: any) => ({ value: b.id, label: b.name }))} />
        </div>
        <div>
          <div style={{ marginBottom: 4, color: '#666', fontSize: 12 }}>选择设备</div>
          <Select style={{ width: 200 }} placeholder="请选择设备" allowClear value={selectedDevice}
            disabled={!selectedBuilding} onChange={(v) => setSelectedDevice(v ? Number(v) : null)}
            options={devices.map((d: any) => ({ value: d.id, label: d.name }))} />
        </div>
        {selectedDevice && <div style={{ paddingBottom: 4, color: '#006875', fontWeight: 500 }}>共 {data.length} 条属性</div>}
      </div>
      <Table rowKey="id" columns={cols} dataSource={data} loading={loading} scroll={{ x: 950 }}
        locale={{ emptyText: selectedDevice ? '暂无属性' : '请先选择项目、楼宇、设备' }} />
      <Modal title={editing ? '编辑' : '新增'} open={modalOpen} onOk={form.submit} onCancel={() => setModalOpen(false)}>
        <Form form={form} layout="vertical" onFinish={save}>
          <Form.Item name="prop_name" label="属性名" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="prop_short" label="属性简称"><Input /></Form.Item>
          <Form.Item name="prop_value" label="属性值"><Input /></Form.Item>
          <Form.Item name="unit" label="单位"><Input /></Form.Item>
          <Form.Item name="operation_type" label="操作类型">
            <Select options={OP_TYPES.map(t => ({ value: t, label: t }))} />
          </Form.Item>
          <Form.Item name="min_value" label="最小值"><Input /></Form.Item>
          <Form.Item name="max_value" label="最大值"><Input /></Form.Item>
          <Form.Item name="is_key" label="关键属性" valuePropName="checked"><Switch checkedChildren="是" unCheckedChildren="否" /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}