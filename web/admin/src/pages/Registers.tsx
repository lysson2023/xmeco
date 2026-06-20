import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Select, Space, message, Popconfirm, InputNumber, Row, Col } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import api from '../api/client';

const READ_CODES = ['01', '02', '03', '04'];
const WRITE_CODES = ['05', '06', '10'];
const DATA_ORDERS = ['高位在前', '低位在前', '低字在前'];
const DATA_LENGTHS = [2, 4, 6, 8];
const DATA_TYPES = ['无符号16位整数','有符号16位整数','无符号32位整数','有符号32位整数','无符号16位小数','有符号16位小数','无符号32位小数','有符号32位小数','单精度浮点数'];
const MAGNIFICATIONS = [0.0001, 0.001, 0.01, 0.1, 1, 10, 100, 1000];

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
  const [selProject, setSelProject] = useState<number | null>(null);
  const [selBuilding, setSelBuilding] = useState<number | null>(null);
  const [selDevice, setSelDevice] = useState<number | null>(null);
  const [selProperty, setSelProperty] = useState<number | null>(null);
  const [customReadCode, setCustomReadCode] = useState(false);
  const [customWriteCode, setCustomWriteCode] = useState(false);
  const [customMag, setCustomMag] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => { api.get('/projects').then(r => setProjects(r.data)); }, []);
  useEffect(() => { api.get('/buildings').then(r => setAllBuildings(r.data)); }, []);

  useEffect(() => {
    if (selProject) { setBuildings(allBuildings.filter((b:any)=>Number(b.project_id)===Number(selProject))); setSelBuilding(null); setSelDevice(null); setSelProperty(null); }
    else { setBuildings([]); setSelBuilding(null); setSelDevice(null); setSelProperty(null); }
  }, [selProject]);
  useEffect(() => {
    if (selBuilding) { api.get('/devices?building_id='+selBuilding).then(r=>setDevices(r.data)); setSelDevice(null); setSelProperty(null); }
    else { setDevices([]); setSelDevice(null); setSelProperty(null); }
  }, [selBuilding]);
  useEffect(() => {
    if (selDevice) { api.get('/properties?device_id='+selDevice).then(r=>setProperties(r.data)); setSelProperty(null); }
    else { setProperties([]); setSelProperty(null); }
  }, [selDevice]);
  useEffect(() => {
    if (selProperty) { setLoading(true); api.get('/registers?property_id='+selProperty).then(r=>{setData(r.data);setLoading(false);}); }
    else { setData([]); }
  }, [selProperty]);

  const save = async (v: any) => {
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
  };
  const del = async (id: number) => { await api.delete('/registers/'+id); message.success('已删除');
    if (selProperty) api.get('/registers?property_id='+selProperty).then(r=>setData(r.data)); };

  const cols = [
    { title: 'ID', dataIndex: 'id', width: 45 },
    { title: '名称', dataIndex: 'name', width: 100, render: (v:string)=>v||'-' },
    { title: '读地址', dataIndex: 'read_addr', width: 65 },
    { title: '写地址', dataIndex: 'write_addr', width: 65, render: (v:any)=>v||'-' },
    { title: '指令名称', dataIndex: 'command_name', width: 90, render: (v:string)=>v||'-' },
    { title: '指令码', dataIndex: 'command_code', width: 70, render: (v:string)=>v||'-' },
    { title: '读码', dataIndex: 'read_code', width: 45 },
    { title: '写码', dataIndex: 'write_code', width: 45 },
    { title: '数据类型', dataIndex: 'data_type', width: 110, ellipsis: true },
    { title: '长度', dataIndex: 'data_length', width: 45 },
    { title: '倍率', dataIndex: 'magnification', width: 55 },
    { title: '操作', width: 90, render: (_:any, r:any) => (
      <Space size="small">
        <a onClick={() => { setEditing(r); form.setFieldsValue(r);
          setCustomReadCode(!READ_CODES.includes(r.read_code));
          setCustomWriteCode(!WRITE_CODES.includes(r.write_code));
          setCustomMag(!MAGNIFICATIONS.includes(Number(r.magnification)));
          setModalOpen(true); }}>编辑</a>
        <Popconfirm title="确定?" onConfirm={()=>del(r.id)}><a style={{color:'red'}}>删除</a></Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
        <h2>寄存器</h2>
        <Button type="primary" icon={<PlusOutlined />} disabled={!selProperty}
          onClick={()=>{setEditing(null);setCustomReadCode(false);setCustomWriteCode(false);setCustomMag(false);form.resetFields();setModalOpen(true);}}>新增</Button>
      </div>
      <div style={{ display: 'flex', gap: 10, marginBottom: 12, alignItems: 'flex-end' }}>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>项目</div><Select style={{width:155}} placeholder="项目" allowClear value={selProject} onChange={v=>setSelProject(v?Number(v):null)} options={projects.map(p=>({value:p.id,label:p.name}))} /></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>楼宇</div><Select style={{width:165}} placeholder="楼宇" allowClear value={selBuilding} disabled={!selProject} onChange={v=>setSelBuilding(v?Number(v):null)} options={buildings.map(b=>({value:b.id,label:b.name}))} /></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>设备</div><Select style={{width:155}} placeholder="设备" allowClear value={selDevice} disabled={!selBuilding} onChange={v=>setSelDevice(v?Number(v):null)} options={devices.map(d=>({value:d.id,label:d.name}))} /></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>属性</div><Select style={{width:155}} placeholder="属性" allowClear value={selProperty} disabled={!selDevice} onChange={v=>setSelProperty(v?Number(v):null)} options={properties.map(p=>({value:p.id,label:p.prop_name}))} /></div>
        {selProperty && <span style={{paddingBottom:2,color:'#006875',fontWeight:500}}>共 {data.length} 条</span>}
      </div>
      <Table rowKey="id" columns={cols} dataSource={data} loading={loading} scroll={{x:1000}} size="small"
        locale={{emptyText:selProperty?'暂无寄存器':'请依次选择项目→楼宇→设备→属性'}} />
      <Modal title={editing?'编辑':'新增'} width={700} open={modalOpen} onOk={form.submit} onCancel={()=>{setModalOpen(false);setCustomReadCode(false);setCustomWriteCode(false);setCustomMag(false);}}>
        <Form form={form} layout="vertical" onFinish={save}>
          <Row gutter={16}>
            <Col span={12}><Form.Item name="name" label="名称"><Input /></Form.Item></Col>
            <Col span={12}><Form.Item name="command_name" label="指令名称"><Input /></Form.Item></Col>
            <Col span={12}><Form.Item name="read_addr" label="读地址" rules={[{required:true}]}><InputNumber style={{width:'100%'}} /></Form.Item></Col>
            <Col span={12}><Form.Item name="write_addr" label="写地址"><InputNumber style={{width:'100%'}} /></Form.Item></Col>
            <Col span={12}><Form.Item name="command_code" label="指令码"><Input /></Form.Item></Col>
            <Col span={12}><Form.Item name="status_code" label="状态码"><Input /></Form.Item></Col>
            <Col span={8}><Form.Item name="data_order" label="数据顺序"><Select options={DATA_ORDERS.map(t=>({value:t,label:t}))} /></Form.Item></Col>
            <Col span={8}><Form.Item name="data_length" label="数据长度"><Select options={DATA_LENGTHS.map(t=>({value:t,label:String(t)}))} /></Form.Item></Col>
            <Col span={8}><Form.Item name="data_type" label="数据类型"><Select options={DATA_TYPES.map(t=>({value:t,label:t}))} /></Form.Item></Col>
            <Col span={8}><Form.Item name="read_code" label="读功能码">
              <Select onChange={(v:any)=>setCustomReadCode(v==='__other__')} options={[...READ_CODES.map(t=>({value:t,label:t})),{value:'__other__',label:'其它'}]} /></Form.Item></Col>
            {customReadCode && <Col span={8}><Form.Item name="custom_read_code" label="自定义读码" rules={[{required:true}]}><Input placeholder="输入读功能码" /></Form.Item></Col>}
            <Col span={8}><Form.Item name="write_code" label="写功能码">
              <Select onChange={(v:any)=>setCustomWriteCode(v==='__other__')} options={[...WRITE_CODES.map(t=>({value:t,label:t})),{value:'__other__',label:'其它'}]} /></Form.Item></Col>
            {customWriteCode && <Col span={8}><Form.Item name="custom_write_code" label="自定义写码" rules={[{required:true}]}><Input placeholder="输入写功能码" /></Form.Item></Col>}
            <Col span={8}><Form.Item name="magnification" label="放大倍数">
              <Select onChange={(v:any)=>setCustomMag(v==='__other__')} options={[...MAGNIFICATIONS.map(t=>({value:t,label:String(t)})),{value:'__other__',label:'其它'}]} /></Form.Item></Col>
            {customMag && <Col span={8}><Form.Item name="custom_mag" label="自定义倍率" rules={[{required:true}]}><Input placeholder="输入放大倍数" /></Form.Item></Col>}
            <Col span={12}><Form.Item name="data_mask" label="掩码"><Input /></Form.Item></Col>
          </Row>
        </Form>
      </Modal>
    </div>
  );
}