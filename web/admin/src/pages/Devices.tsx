import { useEffect, useState, useRef } from 'react';
import { Table, Button, Modal, Form, Input, Select, Space, message, Popconfirm, InputNumber } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useSearchParams, useNavigate } from 'react-router-dom';
import api from '../api/client';

const DEVICE_TYPES = ['主机', '冷冻泵', '冷却泵', '冷却塔', '阀门', '二次泵', '电表', '温湿度传感器', '其它'];

export default function Devices() {
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [projects, setProjects] = useState<any[]>([]);
  const [buildings, setBuildings] = useState<any[]>([]);
  const [allBuildings, setAllBuildings] = useState<any[]>([]);
  const [selectedProject, setSelectedProject] = useState<number | null>(null);
  const [selectedBuilding, setSelectedBuilding] = useState<number | null>(null);
  const [customType, setCustomType] = useState(false);
  const [form] = Form.useForm();
  const [searchParams, setSearchParams] = useSearchParams();
  const mounted = useRef(false);
  const navigate = useNavigate();

  useEffect(() => {
    api.get('/projects').then(r => setProjects(r.data));
    api.get('/buildings').then(r => {
      setAllBuildings(r.data);
      const bid = searchParams.get('building_id');
      if (bid) {
        const building = r.data.find((b: any) => Number(b.id) === Number(bid));
        if (building) {
          setSelectedProject(Number(building.project_id));
          setSelectedBuilding(Number(bid));
        }
      }
    });
  }, []);

  useEffect(() => {
    if (selectedProject) {
      const filtered = allBuildings.filter((b: any) => Number(b.project_id) === Number(selectedProject));
      setBuildings(filtered);
      if (!filtered.find((b: any) => Number(b.id) === Number(selectedBuilding))) {
        setSelectedBuilding(null);
      }
    } else {
      setBuildings([]);
      setSelectedBuilding(null);
    }
  }, [selectedProject, allBuildings]);

  useEffect(() => {
    if (selectedBuilding) {
      setSearchParams({ building_id: String(selectedBuilding) });
    } else if (mounted.current) {
      setSearchParams({});
    }
    mounted.current = true;
  }, [selectedBuilding]);

  useEffect(() => {
    if (selectedBuilding) {
      setLoading(true);
      api.get('/devices?building_id='+selectedBuilding).then(r => { setData(r.data); setLoading(false); });
    } else {
      setData([]);
    }
  }, [selectedBuilding]);

  const getBuildingName = (buildingId: number) =>
    allBuildings.find((b: any) => Number(b.id) === buildingId)?.name || '-';

  const getProjectName = (buildingId: number) => {
    const building = allBuildings.find((b: any) => Number(b.id) === buildingId);
    if (!building) return '-';
    return projects.find((p: any) => Number(p.id) === Number(building.project_id))?.name || '-';
  };

  const save = async (v: any) => {
    const payload = { ...v, building_id: selectedBuilding };
    if (payload.device_type === '其它') payload.device_type = payload.custom_device_type;
    delete payload.custom_device_type;
    if (editing) { await api.put('/devices/'+editing.id, payload); message.success('保存成功'); }
    else { await api.post('/devices', payload); message.success('保存成功'); }
    setModalOpen(false); setEditing(null); setCustomType(false); form.resetFields();
    if (selectedBuilding) api.get('/devices?building_id='+selectedBuilding).then(r => setData(r.data));
  };
  const del = async (id: number) => {
    await api.delete('/devices/'+id); message.success('已删除');
    if (selectedBuilding) api.get('/devices?building_id='+selectedBuilding).then(r => setData(r.data));
  };

  const cols = [
    { title: 'ID', dataIndex: 'id', width: 45 },
    { title: '名称', dataIndex: 'name', width: 130 },
    { title: '设备分类', dataIndex: 'device_type', width: 90 },
    { title: '所属项目', dataIndex: 'building_id', width: 100, render: (v: number) => getProjectName(v) },
    { title: '所属楼宇', dataIndex: 'building_id', width: 100, render: (v: number) => getBuildingName(v) },
    { title: '设备地址', dataIndex: 'node_address', width: 70, render: (v: number) => v || '-' },
    { title: '网关名称', dataIndex: 'gateway_type', width: 80, render: (v: string) => v || '-' },
    { title: '电台地址', dataIndex: 'device_no', width: 70, render: (v: number) => v || '-' },
    { title: '网关IMEI', dataIndex: 'gateway_imei', width: 140, ellipsis: true, render: (v: string) => v || '-' },
    { title: '状态', dataIndex: 'online_status', width: 60, render: (v: string) => v || '-' },
    { title: '操作', width: 180, render: (_: any, r: any) => (
      <Space size="small">
        <a onClick={() => { setEditing(r); form.setFieldsValue(r); setCustomType(!DEVICE_TYPES.includes(r.device_type)); setModalOpen(true); }}>编辑</a>
        <a onClick={() => navigate(`/properties?device_id=${r.id}`)}>属性</a>
        <a onClick={() => navigate(`/logs?device_id=${r.id}`)}>日志</a>
        <Popconfirm title="确定删除?" onConfirm={() => del(r.id)}><a style={{ color: 'red' }}>删除</a></Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>设备管理</h2>
        <Button type="primary" icon={<PlusOutlined />} disabled={!selectedBuilding}
          onClick={() => { setEditing(null); setCustomType(false); form.resetFields(); setModalOpen(true); }}>新增</Button>
      </div>
      <div style={{ display: 'flex', gap: 16, marginBottom: 16, alignItems: 'flex-end' }}>
        <div>
          <div style={{ marginBottom: 4, color: '#666', fontSize: 12 }}>选择项目</div>
          <Select style={{ width: 200 }} placeholder="请选择项目" allowClear
            value={selectedProject}
            onChange={(v) => { setSelectedProject(v ? Number(v) : null); }}
            options={projects.map((p: any) => ({ value: p.id, label: p.name }))} />
        </div>
        <div>
          <div style={{ marginBottom: 4, color: '#666', fontSize: 12 }}>选择楼宇</div>
          <Select style={{ width: 220 }} placeholder="请选择楼宇" allowClear
            value={selectedBuilding}
            disabled={!selectedProject}
            onChange={(v) => setSelectedBuilding(v ? Number(v) : null)}
            notFoundContent={selectedProject ? '该项目下暂无楼宇' : '请先选择项目'}
            options={buildings.map((b: any) => ({ value: b.id, label: b.name }))} />
        </div>
        {selectedBuilding && <div style={{ paddingBottom: 4, color: '#006875', fontWeight: 500 }}>共 {data.length} 台设备</div>}
      </div>
      <Table rowKey="id" columns={cols} dataSource={data} loading={loading} scroll={{ x: 1000 }}
        locale={{ emptyText: selectedBuilding ? '暂无设备' : '请先选择项目，再选择楼宇' }} />
      <Modal title={editing ? '编辑' : '新增'} open={modalOpen} onOk={form.submit} onCancel={() => { setModalOpen(false); setCustomType(false); }}>
        <Form form={form} layout="vertical" onFinish={save} initialValues={{ device_type: '主机' }}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="device_type" label="设备分类" rules={[{ required: true }]}>
            <Select onChange={(v: string) => setCustomType(v === '其它')} options={DEVICE_TYPES.map(t => ({ value: t, label: t }))} />
          </Form.Item>
          {customType && <Form.Item name="custom_device_type" label="自定义分类" rules={[{ required: true }]}><Input placeholder="输入自定义设备分类" /></Form.Item>}
          <Form.Item name="node_address" label="设备地址"><InputNumber style={{ width: '100%' }} min={0} /></Form.Item>
          <Form.Item name="gateway_type" label="网关名称"><Input placeholder="如: custom, dtu" /></Form.Item>
          <Form.Item name="device_no" label="电台地址"><InputNumber style={{ width: '100%' }} min={0} /></Form.Item>
          <Form.Item name="gateway_imei" label="网关IMEI"><Input /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}