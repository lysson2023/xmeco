import { useEffect, useState, useRef } from 'react';
import { Table, Button, Modal, Form, Input, Select, DatePicker, Space, message, Popconfirm, Tag, InputNumber } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import api from '../api/client';

const RECORD_TYPES = ['维修', '保养', '巡检', '更换', '其他'];
const STATUSES = ['已完成', '待处理', '处理中'];

export default function MaintenanceRecords() {
  const [records, setRecords] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form] = Form.useForm();

  // Cascade state
  const [projects, setProjects] = useState<any[]>([]);
  const [allBuildings, setAllBuildings] = useState<any[]>([]);
  const [buildings, setBuildings] = useState<any[]>([]);
  const [devices, setDevices] = useState<any[]>([]);
  const [selectedProject, setSelectedProject] = useState<number | null>(null);
  const [selectedBuilding, setSelectedBuilding] = useState<number | null>(null);
  const [selectedDevice, setSelectedDevice] = useState<number | null>(null);
  const fetchSeqRef = useRef(0);

  // Load projects and all buildings on mount
  useEffect(() => { api.get('/projects').then(r => setProjects(r.data || [])); }, []);
  useEffect(() => { api.get('/buildings').then(r => setAllBuildings(r.data || [])); }, []);

  // Cascade: project -> filter buildings
  useEffect(() => {
    if (selectedProject) {
      setBuildings(allBuildings.filter((b: any) => Number(b.project_id) === Number(selectedProject)));
      setSelectedBuilding(null); setSelectedDevice(null);
    } else { setBuildings([]); setSelectedBuilding(null); setSelectedDevice(null); }
  }, [selectedProject, allBuildings]);

  // Cascade: building -> load devices
  useEffect(() => {
    if (selectedBuilding) {
      api.get('/devices?building_id=' + selectedBuilding).then(r => {
        setDevices(r.data || []);
      });
      setSelectedDevice(null);
    } else { setDevices([]); setSelectedDevice(null); }
  }, [selectedBuilding]);

  // Cascade: device -> load maintenance records
  useEffect(() => {
    if (selectedDevice) {
      fetchRecords();
    } else {
      setRecords([]);
    }
  }, [selectedDevice]);

  const fetchRecords = async () => {
    if (!selectedDevice) return;
    setLoading(true);
    const seq = ++fetchSeqRef.current;
    try {
      const r = await api.get('/maintenance-records', { params: { device_id: selectedDevice } });
      if (seq !== fetchSeqRef.current) return;
      setRecords(r.data || []);
    } catch (err: any) {
      if (seq !== fetchSeqRef.current) return;
      if (err?.response?.status !== 401) console.error('加载维保记录失败:', err);
      setRecords([]);
    } finally {
      if (seq === fetchSeqRef.current) setLoading(false);
    }
  };

  const save = async (values: any) => {
    try {
      const payload = {
        ...values,
        device_id: selectedDevice,
        record_date: values.record_date ? values.record_date.format('YYYY-MM-DD') : dayjs().format('YYYY-MM-DD'),
        next_date: values.next_date ? values.next_date.format('YYYY-MM-DD') : null,
      };
      if (editing) {
        await api.put('/maintenance-records/' + editing.id, payload);
        message.success('修改成功');
      } else {
        await api.post('/maintenance-records', payload);
        message.success('创建成功');
      }
      setModalOpen(false);
      setEditing(null);
      form.resetFields();
      fetchRecords();
    } catch (e) { console.error('保存维保记录失败:', e); message.error('保存失败'); }
  };

  const del = async (id: number) => {
    try { await api.delete('/maintenance-records/' + id); message.success('已删除'); fetchRecords(); }
    catch (e) { console.error('删除维保记录失败:', e); message.error('删除失败'); }
  };

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setModalOpen(true);
  };

  const openEdit = (r: any) => {
    setEditing(r);
    form.setFieldsValue({
      ...r,
      record_date: r.record_date ? dayjs(r.record_date) : null,
      next_date: r.next_date ? dayjs(r.next_date) : null,
    });
    setModalOpen(true);
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '维保名称', dataIndex: 'name', width: 120, ellipsis: true },
    { title: '维保类型', dataIndex: 'record_type', width: 80, render: (v: string) => <Tag color="blue">{v || '维修'}</Tag> },
    { title: '维保说明', dataIndex: 'description', width: 200, ellipsis: true },
    { title: '维保公司', dataIndex: 'company', width: 120, ellipsis: true },
    { title: '负责人', dataIndex: 'operator', width: 80 },
    { title: '维保时间', dataIndex: 'record_date', width: 100 },
    { title: '下次维保', dataIndex: 'next_date', width: 100, render: (v: string | null) => v || '-' },
    { title: '费用', dataIndex: 'cost', width: 80, render: (v: number) => v ? `¥${v.toFixed(0)}` : '-' },
    { title: '状态', dataIndex: 'status', width: 80, render: (v: string) => {
      const color: Record<string, string> = { '已完成': 'green', '待处理': 'orange', '处理中': 'blue' };
      return <Tag color={color[v] || 'default'}>{v || '-'}</Tag>;
    }},
    { title: '操作', width: 120, render: (_: any, r: any) => (
      <Space size="small">
        <a onClick={() => openEdit(r)}>编辑</a>
        <Popconfirm title="确定删除?" onConfirm={() => del(r.id)}>
          <a style={{ color: 'red' }}>删除</a>
        </Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>维保记录</h2>
        <Button type="primary" icon={<PlusOutlined />} disabled={!selectedDevice} onClick={openCreate}>
          新增记录
        </Button>
      </div>

      {/* Cascade selectors */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 16, flexWrap: 'wrap' }}>
        <div>
          <div style={{ marginBottom: 4, color: '#666', fontSize: 12 }}>项目</div>
          <Select
            style={{ width: 180 }}
            placeholder="选择项目"
            allowClear
            value={selectedProject}
            onChange={v => { setSelectedProject(v); }}
            options={projects.map((p: any) => ({ value: p.id, label: p.name }))}
          />
        </div>
        <div>
          <div style={{ marginBottom: 4, color: '#666', fontSize: 12 }}>楼宇</div>
          <Select
            style={{ width: 180 }}
            placeholder="选择楼宇"
            allowClear
            value={selectedBuilding}
            onChange={v => { setSelectedBuilding(v); }}
            options={buildings.map((b: any) => ({ value: b.id, label: b.name }))}
            disabled={!selectedProject}
          />
        </div>
        <div>
          <div style={{ marginBottom: 4, color: '#666', fontSize: 12 }}>设备</div>
          <Select
            style={{ width: 200 }}
            placeholder="选择设备"
            allowClear
            value={selectedDevice}
            onChange={v => { setSelectedDevice(v); }}
            options={devices.map((d: any) => ({ value: d.id, label: d.name }))}
            disabled={!selectedBuilding}
          />
        </div>
      </div>

      {/* Table */}
      <Table
        rowKey="id"
        columns={columns}
        dataSource={records}
        loading={loading}
        scroll={{ x: 1200 }}
        locale={{ emptyText: selectedDevice ? '暂无维保记录' : '请选择设备查看维保记录' }}
      />

      {/* Add/Edit Modal */}
      <Modal
        title={editing ? '编辑维保记录' : '新增维保记录'}
        open={modalOpen}
        onOk={form.submit}
        onCancel={() => { setModalOpen(false); setEditing(null); }}
        width={600}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={save}>
          <Form.Item name="name" label="维保名称" rules={[{ required: true, message: '请输入维保名称' }]}>
            <Input placeholder="如：主机季度保养" />
          </Form.Item>
          <Form.Item name="record_type" label="维保类型" initialValue="维修">
            <Select options={RECORD_TYPES.map(t => ({ value: t, label: t }))} />
          </Form.Item>
          <Form.Item name="description" label="维保说明">
            <Input.TextArea rows={2} placeholder="维保工作详细说明" />
          </Form.Item>
          <Form.Item name="company" label="维保公司">
            <Input placeholder="维保服务公司名称" />
          </Form.Item>
          <Form.Item name="operator" label="负责人">
            <Input placeholder="负责人姓名" />
          </Form.Item>
          <Form.Item name="record_date" label="维保时间" rules={[{ required: true, message: '请选择维保时间' }]}>
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="next_date" label="下次维保">
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="cost" label="费用(元)">
            <InputNumber min={0} precision={2} style={{ width: '100%' }} placeholder="0.00" />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue="已完成">
            <Select options={STATUSES.map(s => ({ value: s, label: s }))} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} placeholder="其他备注信息" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
