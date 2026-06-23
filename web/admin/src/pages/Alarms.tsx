import { useEffect, useState, useRef } from 'react';
import { Table, Button, Modal, Form, Input, Select, Space, message, Popconfirm, Row, Col, Tag } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import api from '../api/client';

const MODE_OPTIONS = ['制冷','制热','制冷热水','制热热水','开机','关机','营业模式','非营业模式'];
const OP_NUMERIC = '数值';
const OP_SWITCH = '开关机';
const OP_MODE = '模式选择';

export default function Alarms() {
  const [rules, setRules] = useState<any[]>([]);
  const [logs, setLogs] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [projects, setProjects] = useState<any[]>([]);
  const [allBuildings, setAllBuildings] = useState<any[]>([]);
  const [buildings, setBuildings] = useState<any[]>([]);
  const [devices, setDevices] = useState<any[]>([]);
  const [properties, setProperties] = useState<any[]>([]);
  const [users, setUsers] = useState<any[]>([]);
  const [selProject, setSelProject] = useState<number|null>(null);
  const [selBuilding, setSelBuilding] = useState<number|null>(null);
  const [selDevice, setSelDevice] = useState<number|null>(null);
  const [selProperty, setSelProperty] = useState<number|null>(null);
  const [selPropDetail, setSelPropDetail] = useState<any>(null);
  const [customMode, setCustomMode] = useState(false);
  const [isRange, setIsRange] = useState(false);
  const [form] = Form.useForm();
  const [searchParams, setSearchParams] = useSearchParams();
  const mounted = useRef(false);
  const restoring = useRef(false);

  useEffect(() => {
    api.get('/projects').then(r=>setProjects(r.data));
    api.get('/users').then(r=>{setUsers(r.data);});
    api.get('/buildings').then(r=>{
      setAllBuildings(r.data);
      const did = searchParams.get('device_id');
      if (did) {
        api.get('/devices?building_id=0').then(dr=>{
          const dev = dr.data.find((d:any)=>Number(d.id)===Number(did));
          if (!dev) return;
          const bld = r.data.find((b:any)=>Number(b.id)===Number(dev.building_id));
          if (!bld) return;
          restoring.current = true;
          setSelProject(Number(bld.project_id));
          setSelBuilding(Number(dev.building_id));
          setSelDevice(Number(did));
        });
      }
    });
  }, []);

  useEffect(() => { if(selProject){setBuildings(allBuildings.filter((b:any)=>Number(b.project_id)===Number(selProject)));if(!restoring.current){setSelBuilding(null);setSelDevice(null);setSelProperty(null);}}else{setBuildings([]);setSelBuilding(null);setSelDevice(null);setSelProperty(null);}}, [selProject]);
  useEffect(() => { if(selBuilding){api.get('/devices?building_id='+selBuilding).then(r=>setDevices(r.data));if(!restoring.current){setSelDevice(null);setSelProperty(null);}}else{setDevices([]);setSelDevice(null);setSelProperty(null);}}, [selBuilding]);
  useEffect(() => { if(selDevice){api.get('/properties?device_id='+selDevice).then(r=>setProperties(r.data));if(!restoring.current)setSelProperty(null);api.get('/alarm-logs?device_id='+selDevice).then(r=>setLogs(r.data));}else{setProperties([]);setSelProperty(null);setLogs([]);}}, [selDevice]);
  useEffect(() => {
    if(selProperty){var p=properties.find(x=>x.id===selProperty);setSelPropDetail(p);setLoading(true);
      api.get('/alarm-rules?device_id='+selDevice).then(r=>{setRules(r.data);setLoading(false);});
    }else{setRules([]);setSelPropDetail(null);}
  }, [selProperty]);

  useEffect(() => {
    if (selDevice) setSearchParams({ device_id: String(selDevice) });
    else if (mounted.current) setSearchParams({});
    mounted.current = true;
    restoring.current = false;
  }, [selDevice]);

  const save = async (v: any) => {
    var p: any = { name: v.name, device_id: selDevice, property_id: selProperty, enabled: v.enabled!==false,
      device_type: selPropDetail?.prop_type||'', metric: selPropDetail?.prop_name||'',
      condition: selPropDetail?.operation_type===OP_NUMERIC ? 'range' : 'eq', level: v.level||'warning',
      notify_users: JSON.stringify(v.notify_users||[]) };
    if(selPropDetail?.operation_type===OP_NUMERIC){ p.min_value=v.min_value; p.max_value=v.max_value; p.threshold=null; }
    else{ p.target_value = v.target_value==='__other__' ? v.custom_target : v.target_value; p.min_value=null; p.max_value=null; p.threshold=null; }
    if(editing){ await api.put('/alarm-rules/'+editing.id, p); message.success('保存成功'); }
    else { await api.post('/alarm-rules', p); message.success('保存成功'); }
    setModalOpen(false); setEditing(null); setCustomMode(false); form.resetFields();
    if(selDevice) api.get('/alarm-rules?device_id='+selDevice).then(r=>setRules(r.data));
  };
  const del = async (id: number) => { await api.delete('/alarm-rules/'+id); message.success('已删除');
    if(selDevice) api.get('/alarm-rules?device_id='+selDevice).then(r=>setRules(r.data)); };

  const rcols = [
    { title: 'ID', dataIndex: 'id', width: 40 },
    { title: '名称', dataIndex: 'name', width: 100 },
    { title: '属性', dataIndex: 'metric', width: 90, render: (v:any)=>v||'-' },
    { title: '条件', dataIndex: 'condition', width: 50, render: (v:any)=>v||'-' },
    { title: '阈值/目标', dataIndex: 'threshold', width: 80, render: (v:any,r:any)=>r.target_value||v||'-' },
    { title: '最小值', dataIndex: 'min_value', width: 65, render: (v:any)=>v||'-' },
    { title: '最大值', dataIndex: 'max_value', width: 65, render: (v:any)=>v||'-' },
    { title: '级别', dataIndex: 'level', width: 60, render: (v:any)=><Tag color={v==='critical'?'red':v==='warning'?'orange':'blue'}>{v||'warning'}</Tag> },
    { title: '启用', dataIndex: 'enabled', width: 50, render: (v:any)=>v?'开':'关' },
    { title: '操作', width: 90, render: (_:any,r:any)=>(<Space size="small"><a onClick={()=>{setEditing(r);form.setFieldsValue(r);setIsRange(r.condition==='range'||!!r.min_value);setCustomMode(r.target_value&&!MODE_OPTIONS.includes(r.target_value));setModalOpen(true);}}>编辑</a><Popconfirm title="确定?" onConfirm={()=>del(r.id)}><a style={{color:'red'}}>删除</a></Popconfirm></Space>)},
  ];

  return (
    <div>
      <div style={{display:'flex',justifyContent:'space-between',marginBottom:12}}><h2>告警规则</h2>
        <Button type="primary" icon={<PlusOutlined/>} disabled={!selProperty} onClick={()=>{setEditing(null);setCustomMode(false);setIsRange(selPropDetail?.operation_type===OP_NUMERIC);form.resetFields();setModalOpen(true);}}>新增</Button></div>
      <div style={{display:'flex',gap:10,marginBottom:12,alignItems:'flex-end'}}>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>项目</div><Select style={{width:155}} placeholder="项目" allowClear value={selProject} onChange={v=>setSelProject(v?Number(v):null)} options={projects.map(p=>({value:p.id,label:p.name}))}/></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>楼宇</div><Select style={{width:165}} placeholder="楼宇" allowClear value={selBuilding} disabled={!selProject} onChange={v=>setSelBuilding(v?Number(v):null)} options={buildings.map(b=>({value:b.id,label:b.name}))}/></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>设备</div><Select style={{width:155}} placeholder="设备" allowClear value={selDevice} disabled={!selBuilding} onChange={v=>setSelDevice(v?Number(v):null)} options={devices.map(d=>({value:d.id,label:d.name}))}/></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>属性</div><Select style={{width:155}} placeholder="属性" allowClear value={selProperty} disabled={!selDevice} onChange={v=>setSelProperty(v?Number(v):null)} options={properties.map(p=>({value:p.id,label:p.prop_name}))}/></div>
        {selProperty && <span style={{paddingBottom:2,color:'#006875',fontWeight:500}}>{selPropDetail?.operation_type} 共{rules.length}条</span>}
      </div>
      <Table rowKey="id" columns={rcols} dataSource={rules} loading={loading} scroll={{x:950}} size="small"
        locale={{emptyText:selProperty?'暂无规则':'请依次选择项目→楼宇→设备→属性'}} style={{marginBottom:24}}/>
      <h2 style={{marginBottom:12}}>告警日志</h2>
      <Table rowKey="id" columns={[
        { title:'ID', dataIndex:'id', width:40 },
        { title:'设备', dataIndex:'device_name', width:120 },
        { title:'类型', dataIndex:'alarm_type', width:80 },
        { title:'级别', dataIndex:'level', width:60, render: (v: any) => <Tag color={v==='critical'?'red':v==='warning'?'orange':'blue'}>{v}</Tag> },
        { title:'消息', dataIndex:'message', width:200, ellipsis:true },
        { title:'值', dataIndex:'value', width:80 },
        { title:'阈值', dataIndex:'threshold', width:80 },
        { title:'时间', dataIndex:'created_at', width:160, render: (v: any) => v ? v.slice(0,19) : '-' },
        { title:'确认', dataIndex:'ack_at', width:100, render: (v: any) => v ? <Tag color="green">已确认</Tag> : <Tag color="red">未确认</Tag> },
      ]} dataSource={logs} loading={loading} scroll={{x:1000}} size="small"
        locale={{emptyText: selDevice ? '暂无告警日志' : '请选择设备查看日志'}} />
      <Modal title={editing?'编辑':'新增'} width={600} open={modalOpen} onOk={form.submit} onCancel={()=>{setModalOpen(false);setCustomMode(false);}}>
        <Form form={form} layout="vertical" onFinish={save} initialValues={{level:'warning',enabled:true}}>
          <Row gutter={16}>
            <Col span={12}><Form.Item name="name" label="名称"><Input/></Form.Item></Col>
            <Col span={12}><Form.Item name="level" label="级别"><Select options={[{value:'warning',label:'警告'},{value:'critical',label:'严重'},{value:'info',label:'信息'}]}/></Form.Item></Col>
            {isRange && <>
              <Col span={12}><Form.Item name="min_value" label="最小值"><Input placeholder="如 200"/></Form.Item></Col>
              <Col span={12}><Form.Item name="max_value" label="最大值"><Input placeholder="如 240"/></Form.Item></Col>
            </>}
            {!isRange && (selPropDetail?.operation_type===OP_SWITCH || editing?.target_value) && <Col span={12}><Form.Item name="target_value" label="目标值"><Select options={[{value:'开机',label:'开机'},{value:'关机',label:'关机'}]}/></Form.Item></Col>}
            {!isRange && (selPropDetail?.operation_type===OP_MODE || (editing?.target_value && selPropDetail?.operation_type!==OP_SWITCH)) && <>
              <Col span={12}><Form.Item name="target_value" label="目标值"><Select onChange={(v:any)=>setCustomMode(v==='__other__')} options={[...MODE_OPTIONS.map(t=>({value:t,label:t})),{value:'__other__',label:'其它'}]}/></Form.Item></Col>
              {customMode && <Col span={12}><Form.Item name="custom_target" label="自定义模式" rules={[{required:true}]}><Input placeholder="输入模式名称"/></Form.Item></Col>}
            </>}
            <Col span={12}><Form.Item name="enabled" label="启用" valuePropName="checked"><Select options={[{value:true,label:'开启'},{value:false,label:'关闭'}]}/></Form.Item></Col>
            <Col span={24}><Form.Item name="notify_users" label="通知用户"><Select mode="multiple" placeholder="选择接收告警的用户" options={users.map((u:any)=>({value:u.id,label:u.username}))}/></Form.Item></Col>
          </Row>
        </Form>
      </Modal>
    </div>
  );
}