import { useEffect, useState, useRef } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Select, Space, message, Popconfirm, Switch } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
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
  const [allRegisters, setAllRegisters] = useState<any[]>([]);
  const [selectedProject, setSelectedProject] = useState<number | null>(null);
  const [selectedBuilding, setSelectedBuilding] = useState<number | null>(null);
  const [selectedDevice, setSelectedDevice] = useState<number | null>(null);
  const [form] = Form.useForm();
  const [sensorForm] = Form.useForm();
  const [sensorModalOpen, setSensorModalOpen] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();
  const mounted = useRef(false);
  const restoring = useRef(false);

  useEffect(() => { api.get('/projects').then(r => setProjects(r.data)); }, []);
  useEffect(() => { api.get('/buildings').then(r => setAllBuildings(r.data)); }, []);
  useEffect(() => { api.get('/registers').then(r => setAllRegisters(r.data)); }, []);

  useEffect(() => {
    if (selectedProject) {
      setBuildings(allBuildings.filter((b: any) => Number(b.project_id) === Number(selectedProject)));
      if (!restoring.current) {
        setSelectedBuilding(null);
        setSelectedDevice(null);
      }
    } else { setBuildings([]); setSelectedBuilding(null); setSelectedDevice(null); }
  }, [selectedProject, allBuildings]);

  useEffect(() => {
    if (selectedBuilding) {
      api.get('/devices?building_id='+selectedBuilding).then(r => setDevices(r.data));
      if (!restoring.current) setSelectedDevice(null);
    } else { setDevices([]); setSelectedDevice(null); }
  }, [selectedBuilding]);

  const [selectedDeviceInfo, setSelectedDeviceInfo] = useState<any>(null);
  const [sensorData, setSensorData] = useState<any[]>([]);
  const devFetchSeq = useRef(0);

  const isSensor = selectedDeviceInfo?.device_type === '温湿度传感器';

  useEffect(() => {
    if (selectedDevice) {
      setLoading(true);
      const seq = ++devFetchSeq.current; // 请求序号，防止快速切换设备时旧请求覆盖新数据
      // 温湿度传感器走专用 API，其他设备走常规属性 API
      api.get('/devices/' + selectedDevice).then(r => {
        if (seq !== devFetchSeq.current) return; // 不是最新请求，丢弃
        setSelectedDeviceInfo(r.data);
        if (r.data?.device_type === '温湿度传感器') {
          // 只加载当前选中的传感器的数据
          api.get('/devices/' + selectedDevice + '/sensor-data').then(r2 => {
            if (seq !== devFetchSeq.current) return;
            const d = r2.data || {};
            setSensorData([{ ...d, device_name: r.data.name, building_id: r.data.building_id }]);
            setData([]);
            setLoading(false);
          }).catch(() => { if (seq !== devFetchSeq.current) return; setSensorData([]); setLoading(false); });
        } else {
          api.get('/properties?device_id=' + selectedDevice).then(r2 => {
            if (seq !== devFetchSeq.current) return;
            setData(r2.data); setLoading(false);
          }).catch(() => { if (seq !== devFetchSeq.current) return; setLoading(false); });
        }
      }).catch(() => { if (seq !== devFetchSeq.current) return; setSelectedDeviceInfo(null); setLoading(false); });
      restoring.current = false;
    } else { setData([]); setSelectedDeviceInfo(null); setSensorData([]); }
  }, [selectedDevice]);

  // Read device_id from URL on initial load and cascade restore
  useEffect(() => {
    const did = searchParams.get('device_id');
    if (did && allBuildings.length > 0) {
      api.get('/devices?building_id=0').then(r => {
        const allDevs = r.data;
        const dev = allDevs.find((d: any) => Number(d.id) === Number(did));
        if (!dev) return;
        const bld = allBuildings.find((b: any) => Number(b.id) === Number(dev.building_id));
        if (!bld) return;
        restoring.current = true;
        setSelectedProject(Number(bld.project_id));
        setSelectedBuilding(Number(dev.building_id));
        setSelectedDevice(Number(did));
      });
    }
  }, [searchParams, allBuildings]);

  // Sync selectedDevice to URL
  useEffect(() => {
    if (selectedDevice) {
      setSearchParams({ device_id: String(selectedDevice) });
    } else if (mounted.current) {
      setSearchParams({});
    }
    mounted.current = true;
  }, [selectedDevice]);

  const getDeviceName = (deviceId: number) =>
    devices.find((d: any) => Number(d.id) === deviceId)?.name || '-';

  const getBldName = (deviceId: number) => {
    const dev = devices.find((d: any) => Number(d.id) === deviceId);
    if (!dev) return '-';
    return allBuildings.find((b: any) => Number(b.id) === Number(dev.building_id))?.name || '-';
  };

  const getProjName = (deviceId: number) => {
    const dev = devices.find((d: any) => Number(d.id) === deviceId);
    if (!dev) return '-';
    const bld = allBuildings.find((b: any) => Number(b.id) === Number(dev.building_id));
    if (!bld) return '-';
    return projects.find((p: any) => Number(p.id) === Number(bld.project_id))?.name || '-';
  };

  const registerCount = (propId: number) =>
    allRegisters.filter((r: any) => Number(r.property_id) === propId).length;

  const save = async (v: any) => {
    try {
      const payload = { ...v, device_id: selectedDevice, is_key: v.is_key || false };
      if (editing) { await api.put('/properties/'+editing.id, payload); message.success('保存成功'); }
      else { await api.post('/properties', payload); message.success('保存成功'); }
      setModalOpen(false); setEditing(null); form.resetFields();
      if (selectedDevice) api.get('/properties?device_id='+selectedDevice).then(r => setData(r.data));
    } catch { message.error('保存失败'); }
  };
  const del = async (id: number) => {
    try {
      await api.delete('/properties/'+id); message.success('已删除');
      if (selectedDevice) api.get('/properties?device_id='+selectedDevice).then(r => setData(r.data));
    } catch { message.error('删除失败'); }
  };

  const cols = [
    { title: 'ID', dataIndex: 'id', width: 45 },
    { title: '属性名', dataIndex: 'prop_name', width: 120, ellipsis: true },
    { title: '所属设备', dataIndex: 'device_id', width: 100, ellipsis: true, render: (v: number) => getDeviceName(v) },
    { title: '所属楼宇', dataIndex: 'device_id', width: 100, ellipsis: true, render: (v: number) => getBldName(v) },
    { title: '所属项目', dataIndex: 'device_id', width: 100, ellipsis: true, render: (v: number) => getProjName(v) },
    { title: '属性值', dataIndex: 'prop_value', width: 80, render: (v: string) => v || '-' },
    { title: '单位', dataIndex: 'unit', width: 55, render: (v: string) => v || '-' },
    { title: '操作类型', dataIndex: 'operation_type', width: 85 },
    { title: '寄存器', width: 70, render: (_: any, r: any) => {
      const n = registerCount(r.id);
      return n > 0 ? <span style={{fontWeight:500,color:'#006875'}}>{n}</span> : <span style={{color:'#999'}}>0</span>;
    }},
    { title: '关键属性', dataIndex: 'is_key', width: 80, render: (v: boolean) => v ? '是' : '否' },
    { title: '简称', dataIndex: 'prop_short', width: 70, render: (v: string) => v || '-' },
    { title: '电能方向', width: 90, hidden: selectedDeviceInfo?.device_type !== '电表', render: (_: any, r: any) => (
      <Select
        size="small"
        style={{ width: 70 }}
        value={selectedDeviceInfo?.power_sign === -1 ? -1 : 1}
        onChange={async (v) => {
          try {
            const dev = { ...selectedDeviceInfo, power_sign: v };
            await api.put('/devices/' + selectedDevice, dev);
            setSelectedDeviceInfo(dev);
            message.success('电能方向已更新');
          } catch { message.error('更新失败'); }
        }}
        options={[{ value: 1, label: '+ 加' }, { value: -1, label: '- 减' }]}
      />
    )},
    { title: '操作', width: 100, render: (_: any, r: any) => (
      <Space>
        <a onClick={() => { setEditing(r); form.setFieldsValue(r); setModalOpen(true); }}>编辑</a>
        <Popconfirm title="确定删除?" onConfirm={() => del(r.id)}><a style={{ color: 'red' }}>删除</a></Popconfirm>
      </Space>
    )},
  ];

  // 温湿度传感器专用列
  const sensorCols = [
    { title: 'ID', dataIndex: 'device_id', width: 50 },
    { title: '属性名', dataIndex: 'device_name', width: 100, ellipsis: true },
    { title: '所属设备', dataIndex: 'device_name', width: 100, ellipsis: true },
    { title: '所属楼宇', dataIndex: 'building_id', width: 100, ellipsis: true, render: (v: number) => getBldName(v) },
    { title: '所属项目', dataIndex: 'building_id', width: 100, ellipsis: true, render: (v: number) => getProjName(v) },
    { title: '网关编号', dataIndex: 'gateway_imei', width: 120, ellipsis: true, render: (v: string) => v || '-' },
    { title: '传感器通道号', dataIndex: 'channel_no', width: 110 },
    { title: '传感器编号', dataIndex: 'sensor_no', width: 110, ellipsis: true, render: (v: string) => v || '-' },
    { title: '时间间隔(M)', dataIndex: 'interval_minutes', width: 110 },
    { title: '传感器类型', dataIndex: 'sensor_type', width: 90, ellipsis: true, render: (v: string) => v || '-' },
    { title: '温度(℃)', dataIndex: 'temperature', width: 85, render: (v: number) => v ? v.toFixed(1) : '-' },
    { title: '湿度(%)', dataIndex: 'humidity', width: 85, render: (v: number) => v ? v.toFixed(1) : '-' },
    { title: '电压(V)', dataIndex: 'voltage', width: 85, render: (v: number) => v ? v.toFixed(1) : '-' },
    { title: '信号强度(dbm)', dataIndex: 'signal_strength', width: 110, render: (v: number) => v ? v.toFixed(0) : '-' },
    { title: '更新时间', dataIndex: 'update_time', width: 160, render: (v: string) => v || '-' },
    { title: '操作', width: 80, render: (_: any, r: any) => (
      <a onClick={() => {
        setSensorModalOpen(true);
        sensorForm.setFieldsValue({
          channel_no: r.channel_no || 1,
          sensor_no: r.sensor_no || '',
          interval_minutes: r.interval_minutes || 5,
          temperature: r.temperature ?? undefined,
          humidity: r.humidity ?? undefined,
          voltage: r.voltage ?? undefined,
          signal_strength: r.signal_strength ?? undefined,
        });
      }}>编辑</a>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>{isSensor ? '温湿度传感器' : '属性配置'}</h2>
        <Button type="primary" icon={<PlusOutlined />} disabled={!selectedDevice}
          onClick={() => {
            if (isSensor) {
              // 温湿度传感器：打开传感器配置弹窗
              setSensorModalOpen(true);
              sensorForm.resetFields();
              sensorForm.setFieldsValue({
                channel_no: 1,
                sensor_no: '',
                interval_minutes: 5,
              });
            } else {
              setEditing(null); form.resetFields(); setModalOpen(true);
            }
          }}>新增</Button>
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
        {selectedDevice && <div style={{ paddingBottom: 4, color: '#006875', fontWeight: 500 }}>
          共 {isSensor ? sensorData.length : data.length} {isSensor ? '条传感器数据' : '条属性'}
        </div>}
      </div>
      {isSensor ? (
        <Table rowKey="device_id" columns={sensorCols} dataSource={sensorData} loading={loading} scroll={{ x: 1600 }}
          locale={{ emptyText: selectedDevice ? '暂无传感器数据' : '请先选择项目、楼宇、设备' }} />
      ) : (
        <Table rowKey="id" columns={cols} dataSource={data} loading={loading} scroll={{ x: 950 }}
          locale={{ emptyText: selectedDevice ? '暂无属性' : '请先选择项目、楼宇、设备' }} />
      )}
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
          {selectedDeviceInfo?.device_type === '电表' && (
            <Form.Item label="电能方向" extra="用于大屏总电能计算：加=累加，减=扣减">
              <Select
                value={selectedDeviceInfo?.power_sign === -1 ? -1 : 1}
                onChange={async (v) => {
                  try {
                    const dev = { ...selectedDeviceInfo, power_sign: v };
                    await api.put('/devices/' + selectedDevice, dev);
                    setSelectedDeviceInfo(dev);
                  } catch { message.error('更新失败'); }
                }}
                options={[{ value: 1, label: '+ 加' }, { value: -1, label: '- 减' }]}
              />
            </Form.Item>
          )}
        </Form>
      </Modal>
      {/* 温湿度传感器配置弹窗 */}
      <Modal title="传感器配置" open={sensorModalOpen}
        onOk={() => sensorForm.submit()}
        onCancel={() => setSensorModalOpen(false)}>
        <Form form={sensorForm} layout="vertical" onFinish={async (v) => {
          try {
            await api.put('/devices/' + selectedDevice + '/sensor-config', v);
            message.success('传感器配置已保存');
            setSensorModalOpen(false);
            // 只刷新当前传感器的数据
            if (selectedDevice) {
              const [devRes, sensorRes] = await Promise.all([
                api.get('/devices/' + selectedDevice),
                api.get('/devices/' + selectedDevice + '/sensor-data'),
              ]);
              setSelectedDeviceInfo(devRes.data);
              const d = sensorRes.data || {};
              setSensorData([{ ...d, device_name: devRes.data.name, building_id: devRes.data.building_id }]);
            }
          } catch { message.error('保存失败'); }
        }}>
          <Form.Item name="channel_no" label="传感器通道号" rules={[{ required: true }]}>
            <InputNumber style={{ width: '100%' }} min={1} />
          </Form.Item>
          <Form.Item name="sensor_no" label="传感器编号">
            <Input placeholder="如：TH-001" />
          </Form.Item>
          <Form.Item name="interval_minutes" label="时间间隔(分钟)" rules={[{ required: true }]}>
            <InputNumber style={{ width: '100%' }} min={1} />
          </Form.Item>
          <Form.Item name="temperature" label="温度(℃)"><InputNumber style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="humidity" label="湿度(%)"><InputNumber style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="voltage" label="电压(V)"><InputNumber style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="signal_strength" label="信号强度(dbm)"><InputNumber style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}