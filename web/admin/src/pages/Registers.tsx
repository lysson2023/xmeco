import { useEffect, useState, useRef } from 'react';
import { Table, Button, Modal, Form, Input, Select, Space, message, Popconfirm, InputNumber, Row, Col } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import api from '../api/client';
import { READ_CODES, WRITE_CODES, DATA_ORDERS, REG_COUNTS, DATA_TYPES, MAGNIFICATIONS } from '../utils/constants';

export default function Registers() {
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [projects, setProjects] = useState<any[]>([]);
  const [allBuildings, setAllBuildings] = useState<any[]>([]);
  const [buildings, setBuildings] = useState<any[]>([]);
  const [devices, setDevices] = useState<any[]>([]);
  const [properties, setProperties] = useState<any[]>([]);
  const [allDevices, setAllDevices] = useState<any[]>([]);
  const [allProperties, setAllProperties] = useState<any[]>([]);
  const [selProject, setSelProject] = useState<number | null>(null);
  const [selBuilding, setSelBuilding] = useState<number | null>(null);
  const [selDevice, setSelDevice] = useState<number | null>(null);
  const [selProperty, setSelProperty] = useState<number | null>(null);
  const [customReadCode, setCustomReadCode] = useState(false);
  const [customWriteCode, setCustomWriteCode] = useState(false);
  const [customMag, setCustomMag] = useState(false);
  const [form] = Form.useForm();
  const [searchParams, setSearchParams] = useSearchParams();
  const mounted = useRef(false);

  const restoring = useRef(false);

  useEffect(() => { api.get('/projects').then(r => setProjects(r.data)); }, []);
  useEffect(() => { api.get('/buildings').then(r => setAllBuildings(r.data)); }, []);
  useEffect(() => { api.get('/devices').then(r => setAllDevices(r.data)); }, []);
  useEffect(() => { api.get('/properties?device_id=0').then(r => setAllProperties(r.data)); }, []);

  const getDeviceNameById = (deviceId: number) => allDevices.find((d: any) => Number(d.id) === deviceId)?.name || '-';

  useEffect(() => {
    if (selProject) { setBuildings(allBuildings.filter((b:any)=>Number(b.project_id)===Number(selProject))); if (!restoring.current) { setSelBuilding(null); setSelDevice(null); setSelProperty(null); } }
    else { setBuildings([]); setSelBuilding(null); setSelDevice(null); setSelProperty(null); }
  }, [selProject]);
  useEffect(() => {
    if (selBuilding) { api.get('/devices?building_id='+selBuilding).then(r=>setDevices(r.data)); if (!restoring.current) { setSelDevice(null); setSelProperty(null); } }
    else { setDevices([]); setSelDevice(null); setSelProperty(null); }
  }, [selBuilding]);
  useEffect(() => {
    if (selDevice) { api.get('/properties?device_id='+selDevice).then(r=>setProperties(r.data)); if (!restoring.current) setSelProperty(null); }
    else { setProperties([]); setSelProperty(null); }
  }, [selDevice]);
  useEffect(() => {
    if (selProperty) { setLoading(true); api.get('/registers?property_id='+selProperty).then(r=>{setData(r.data);setLoading(false);}).catch(()=>setLoading(false)); restoring.current = false; }
    else { setData([]); }
  }, [selProperty]);

  useEffect(() => {
    if (selProperty) setSearchParams({ property_id: String(selProperty) });
    else if (mounted.current) setSearchParams({});
    mounted.current = true;
  }, [selProperty]);

  // Read property_id from URL on initial load and cascade restore
  useEffect(() => {
    const pid = searchParams.get('property_id');
    if (pid && allBuildings.length > 0 && allDevices.length > 0 && allProperties.length > 0) {
      const prop = allProperties.find((p: any) => Number(p.id) === Number(pid));
      if (!prop) return;
      const dev = allDevices.find((d: any) => Number(d.id) === Number(prop.device_id));
      if (!dev) return;
      const bld = allBuildings.find((b: any) => Number(b.id) === Number(dev.building_id));
      if (!bld) return;
      restoring.current = true;
      setSelProject(Number(bld.project_id));
      setSelBuilding(Number(dev.building_id));
      setSelDevice(Number(dev.id));
      setSelProperty(Number(pid));
    }
  }, [searchParams, allBuildings, allDevices, allProperties]);

  const save = async (v: any) => {
    try {
      const p: any = { name: v.name, read_addr: v.read_addr, write_addr: v.write_addr, command_name: v.command_name,
        command_code: v.command_code, status_code: v.status_code, data_order: v.data_order, data_length: v.data_length,
        data_mask: v.data_mask, data_type: v.data_type, property_id: selProperty,
        read_code: customReadCode ? v.custom_read_code : v.read_code,
        write_code: customWriteCode ? v.custom_write_code : v.write_code,
        magnification: customMag ? parseFloat(v.custom_mag) : v.magnification };
      if (editing) { await api.put('/registers/'+editing.id, p); message.success('保存成功'); }
      else { await api.post('/registers', p); message.success('保存成功'); }
      setModalOpen(false); setEditing(null); setCustomReadCode(false); setCustomWriteCode(false); setCustomMag(false); form.resetFields();
      if (selProperty) api.get('/registers?property_id='+selProperty).then(r=>setData(r.data));
    } catch { message.error('保存失败'); }
  };
  const del = async (id: number) => {
    try {
      await api.delete('/registers/'+id); message.success('已删除');
      if (selProperty) api.get('/registers?property_id='+selProperty).then(r=>setData(r.data));
    } catch { message.error('删除失败'); }
  };

  const cols = [
    { title: 'ID', dataIndex: 'id', width: 40 },
    { title: '名称', dataIndex: 'name', width: 65, render: (v:string)=>v||'-' },
    { title: '所属属性', dataIndex: 'property_id', width: 65, render: (v:number) => {
      const prop = properties.find((p:any) => Number(p.id) === v);
      return prop?.prop_name || '-';
    }},
    { title: '所属设备', dataIndex: 'property_id', width: 70, render: (_:any, r:any) => {
      const prop = properties.find((p:any) => Number(p.id) === r.property_id);
      return prop ? getDeviceNameById(prop.device_id) : '-';
    }},
    { title: '读地址', dataIndex: 'read_addr', width: 60 },
    { title: '写地址', dataIndex: 'write_addr', width: 58, render: (v:any)=>v||'-' },
    { title: '读码', dataIndex: 'read_code', width: 42 },
    { title: '写码', dataIndex: 'write_code', width: 42 },
    { title: '指令名称', dataIndex: 'command_name', width: 72, render: (v:string)=>v||'-' },
    { title: '指令码', dataIndex: 'command_code', width: 56, render: (v:string)=>v||'-' },
    { title: '状态码', dataIndex: 'status_code', width: 56, ellipsis: true, render: (v:string)=>v||'-' },
    { title: '字节序', dataIndex: 'data_order', width: 58, render: (v:string)=>v||'高位在前' },
    { title: '数量', dataIndex: 'data_length', width: 44 },
    { title: '掩码', dataIndex: 'data_mask', width: 52, render: (v:string)=>v||'-' },
    { title: '数据类型', dataIndex: 'data_type', width: 95, ellipsis: true },
    { title: '倍率', dataIndex: 'magnification', width: 48 },
    { title: '操作', width: 75, render: (_:any, r:any) => (
      <Space size="small">
        <a onClick={() => { setEditing(r); form.setFieldsValue(r);
          setCustomReadCode(!(READ_CODES as readonly string[]).includes(r.read_code));
          setCustomWriteCode(!(WRITE_CODES as readonly string[]).includes(r.write_code));
          setCustomMag(!(MAGNIFICATIONS as readonly number[]).includes(Number(r.magnification)));
          setModalOpen(true); }}>编辑</a>
        <Popconfirm title="确定?" onConfirm={()=>del(r.id)}><a style={{color:'red'}}>删除</a></Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <div style={{display:'flex',justifyContent:'space-between',marginBottom:12}}>
        <h2>寄存器</h2>
        <Button type="primary" icon={<PlusOutlined />} disabled={!selProperty}
          onClick={()=>{setEditing(null);setCustomReadCode(false);setCustomWriteCode(false);setCustomMag(false);form.resetFields();setModalOpen(true);}}>新增</Button>
      </div>
      <div style={{display:'flex',gap:10,marginBottom:12,alignItems:'flex-end'}}>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>项目</div><Select style={{width:155}} placeholder="项目" allowClear value={selProject} onChange={v=>setSelProject(v?Number(v):null)} options={projects.map(p=>({value:p.id,label:p.name}))} /></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>楼宇</div><Select style={{width:165}} placeholder="楼宇" allowClear value={selBuilding} disabled={!selProject} onChange={v=>setSelBuilding(v?Number(v):null)} options={buildings.map(b=>({value:b.id,label:b.name}))} /></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>设备</div><Select style={{width:155}} placeholder="设备" allowClear value={selDevice} disabled={!selBuilding} onChange={v=>setSelDevice(v?Number(v):null)} options={devices.map(d=>({value:d.id,label:d.name}))} /></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>属性</div><Select style={{width:155}} placeholder="属性" allowClear value={selProperty} disabled={!selDevice} onChange={v=>setSelProperty(v?Number(v):null)} options={properties.map(p=>({value:p.id,label:p.prop_name}))} /></div>
        {selProperty && <span style={{paddingBottom:2,color:'#006875',fontWeight:500}}>共 {data.length} 条</span>}
      </div>
      <Table rowKey="id" columns={cols} dataSource={data} loading={loading} scroll={{x:1200}} size="small"
        locale={{emptyText:selProperty?'暂无寄存器':'请依次选择项目→楼宇→设备→属性'}} />
      <Modal title={editing?'编辑':'新增'} width={700} open={modalOpen} onOk={form.submit} onCancel={()=>{setModalOpen(false);setCustomReadCode(false);setCustomWriteCode(false);setCustomMag(false);}}>
        <Form form={form} layout="vertical" onFinish={save}>
          {/* --- 基本信息 --- */}
          <Row gutter={16}>
            <Col span={8}><Form.Item name="name" label="名称"><Input /></Form.Item></Col>
            <Col span={8}><Form.Item name="command_name" label="指令名称"><Input /></Form.Item></Col>
            <Col span={8}><Form.Item name="command_code" label="指令码"><Input /></Form.Item></Col>
          </Row>
          {/* --- 读取配置 --- */}
          <Row gutter={16} style={{marginTop:8}}>
            <Col span={8}><Form.Item name="read_addr" label="读地址" rules={[{required:true}]}><InputNumber style={{width:'100%'}} /></Form.Item></Col>
            <Col span={8}><Form.Item name="read_code" label="读功能码">
              <Select onChange={(v:any)=>setCustomReadCode(v==='__other__')} options={[...READ_CODES.map(t=>({value:t,label:t})),{value:'__other__',label:'其它'}]} /></Form.Item></Col>
            {customReadCode && <Col span={8}><Form.Item name="custom_read_code" label="自定义读码" rules={[{required:true}]}><Input placeholder="输入读功能码" /></Form.Item></Col>}
            <Col span={8}><Form.Item name="data_length" label="寄存器数" tooltip="1 寄存器 = 2 字节"><Select options={REG_COUNTS.map(t=>({value:t,label:String(t)}))} /></Form.Item></Col>
          </Row>
          <Row gutter={16}>
            <Col span={8}><Form.Item name="data_type" label="数据类型"><Select options={DATA_TYPES.map(t=>({value:t,label:t}))} /></Form.Item></Col>
            <Col span={8}><Form.Item name="data_order" label="字节序"><Select options={DATA_ORDERS.map(t=>({value:t,label:t}))} /></Form.Item></Col>
            <Col span={8}><Form.Item name="data_mask" label="掩码"><Input placeholder="如 FFFF" /></Form.Item></Col>
          </Row>
          <Row gutter={16}>
            <Col span={8}><Form.Item name="magnification" label="倍率">
              <Select onChange={(v:any)=>setCustomMag(v==='__other__')} options={[...MAGNIFICATIONS.map(t=>({value:t,label:String(t)})),{value:'__other__',label:'其它'}]} /></Form.Item></Col>
            {customMag && <Col span={8}><Form.Item name="custom_mag" label="自定义倍率" rules={[{required:true}]}><Input placeholder="输入倍率" /></Form.Item></Col>}
          </Row>
          {/* --- 写入配置 --- */}
          <Row gutter={16} style={{marginTop:8}}>
            <Col span={8}><Form.Item name="write_addr" label="写地址"><InputNumber style={{width:'100%'}} /></Form.Item></Col>
            <Col span={8}><Form.Item name="write_code" label="写功能码">
              <Select onChange={(v:any)=>setCustomWriteCode(v==='__other__')} options={[...WRITE_CODES.map(t=>({value:t,label:t})),{value:'__other__',label:'其它'}]} /></Form.Item></Col>
            {customWriteCode && <Col span={8}><Form.Item name="custom_write_code" label="自定义写码" rules={[{required:true}]}><Input placeholder="输入写功能码" /></Form.Item></Col>}
            <Col span={8}><Form.Item name="status_code" label="状态码" tooltip="格式: 01=运行,02=停机"><Input placeholder="01=运行,02=停机" /></Form.Item></Col>
          </Row>
        </Form>
      </Modal>
    </div>
  );
}