import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Space, message, Popconfirm, Select, Cascader } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '../api/client';

interface Project { id: number; name: string; agent_id: number | null; address: string; admin_code: string; city_id: number | null; city_name: string; created_at: string }
interface Building { id: number; project_id: number; name: string }
interface City { id: number; name: string; province: string; admin_code: string }

export default function Projects() {
  const [data, setData] = useState<Project[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Project | null>(null);
  const [agents, setAgents] = useState<any[]>([]);
  const [allUsers, setAllUsers] = useState<any[]>([]);
  const [allBuildings, setAllBuildings] = useState<Building[]>([]);
  const [cascaderOptions, setCascaderOptions] = useState<any[]>([]);
  const [cityMap, setCityMap] = useState<Record<string, City>>({});
  const [assignedUserIds, setAssignedUserIds] = useState<number[]>([]);
  const [form] = Form.useForm();
  const navigate = useNavigate();

  const fetch = async () => {
    setLoading(true);
    try {
      const [p, a, b, provinces, u] = await Promise.all([
        api.get('/projects'),
        api.get('/agents'),
        api.get('/buildings'),
        api.get('/weather/provinces'),
        api.get('/users'),
      ]);
      setData(p.data); setAgents(a.data); setAllBuildings(b.data); setAllUsers(u.data || []);

      // 构建 Cascader 选项树: [省][市]
      const opts: any[] = [];
      const map: Record<string, City> = {};
      for (const prov of provinces.data) {
        const children = prov.cities.map((c: City) => {
          map[c.id] = c;
          return { value: c.id, label: c.name };
        });
        opts.push({ value: prov.province, label: prov.province, children });
      }
      setCascaderOptions(opts);
      setCityMap(map);
    } catch {
      message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { fetch(); }, []);

  const buildingCountByProject = (projectId: number) =>
    allBuildings.filter((b: Building) => Number(b.project_id) === projectId).length;

  const onCityChange = (value: any) => {
    if (value && value.length === 2) {
      const cityId = value[1];
      const city = cityMap[cityId];
      if (city) {
        form.setFieldsValue({
          city_id: city.id,
          city_name: city.name,
          address: city.province + city.name,
          admin_code: city.admin_code,
        });
      }
    }
  };

  const save = async (values: any) => {
    try {
      if (!values.city_id) { values.city_id = null; values.city_name = ''; }
      if (editing) {
        await api.put('/projects/' + editing.id, values);
        await api.put('/projects/' + editing.id + '/users', { user_ids: assignedUserIds });
        message.success('修改成功');
      } else {
        await api.post('/projects', values);
        message.success('创建成功');
      }
      setModalOpen(false); setEditing(null); form.resetFields(); fetch();
    } catch {
      message.error('保存失败');
    }
  };

  const del = async (id: number) => {
    try { await api.delete('/projects/' + id); message.success('已删除'); fetch(); }
    catch { message.error('删除失败'); }
  };

  const openEdit = async (r: Project) => {
    setEditing(r);
    form.setFieldsValue(r);
    if (r.city_id && r.city_name) {
      const city = cityMap[r.city_id];
      if (city) form.setFieldValue('city_cascader', [city.province, r.city_id]);
    }
    // Load assigned user IDs for this project
    try {
      const pu = await api.get('/projects/' + r.id + '/users');
      setAssignedUserIds(pu.data.user_ids || []);
    } catch { setAssignedUserIds([]); }
    setModalOpen(true);
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '项目名称', dataIndex: 'name' },
    { title: '代理商', dataIndex: 'agent_id', render: (v: number | null) => agents.find(a => a.id === v)?.name || '-' },
    { title: '城市', dataIndex: 'city_name', render: (v: string) => v || '-' },
    { title: '地址', dataIndex: 'address', render: (v: string) => v || '-' },
    { title: '行政区划代码', dataIndex: 'admin_code', render: (v: string) => v || '-' },
    {
      title: '楼宇数', width: 80, render: (_: any, r: Project) => {
        const n = buildingCountByProject(r.id);
        return n > 0
          ? <a onClick={() => navigate(`/buildings?project_id=${r.id}`)} style={{ fontWeight: 500 }}>{n}</a>
          : <span style={{ color: '#999' }}>0</span>;
      }
    },
    { title: '创建时间', dataIndex: 'created_at', render: (v: string) => v?.slice(0, 10) },
    {
      title: '操作', render: (_: any, r: Project) => (
        <Space>
          <a onClick={() => openEdit(r)}>编辑</a>
          <Popconfirm title="确定删除?" onConfirm={() => del(r.id)}><a style={{ color: 'red' }}>删除</a></Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>项目管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setModalOpen(true); }}>新建项目</Button>
      </div>
      <Table rowKey="id" columns={columns} dataSource={data} loading={loading} />
      <Modal title={editing ? '编辑' : '新建'} open={modalOpen} onOk={form.submit} onCancel={() => setModalOpen(false)} width={560}>
        <Form form={form} layout="vertical" onFinish={save}>
          <Form.Item name="name" label="项目名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="agent_id" label="代理商"><Select allowClear placeholder="选择代理商" options={agents.map((a: any) => ({ value: a.id, label: a.name }))} /></Form.Item>
          <Form.Item name="city_cascader" label="所在城市" rules={[{ required: true, message: '请选择所在城市' }]}>
            <Cascader
              options={cascaderOptions}
              placeholder="请选择省 → 市"
              onChange={onCityChange}
              showSearch={{ filter: (inputValue, path) => path.some(o => (o.label as string).includes(inputValue)) }}
            />
          </Form.Item>
          <Form.Item name="city_id" hidden><Input /></Form.Item>
          <Form.Item name="city_name" hidden><Input /></Form.Item>
          <Form.Item name="address" label="地址"><Input placeholder="选择城市后自动填充" /></Form.Item>
          <Form.Item name="admin_code" label="行政区划代码"><Input placeholder="选择城市后自动填充" /></Form.Item>
          {editing && (
            <Form.Item label="授权用户" extra="仅选中的用户可在大屏和小程序查看此项目">
              <Select mode="multiple" placeholder="选择用户" value={assignedUserIds}
                onChange={(v) => setAssignedUserIds(v)}
                options={allUsers.map((u: any) => ({ value: u.id, label: u.username }))} />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </div>
  );
}
