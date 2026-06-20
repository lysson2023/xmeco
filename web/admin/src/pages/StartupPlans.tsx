import { useEffect, useState } from 'react';
import { Table, Button, Modal, Form, Input, Select, Space, message, Popconfirm, Row, Col, InputNumber, Tag } from 'antd';
import { PlusOutlined, DeleteOutlined, ArrowUpOutlined, ArrowDownOutlined, PlayCircleOutlined } from '@ant-design/icons';
import api from '../api/client';

export default function StartupPlans() {
  const [plans, setPlans] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [projects, setProjects] = useState<any[]>([]);
  const [allBuildings, setAllBuildings] = useState<any[]>([]);
  const [buildings, setBuildings] = useState<any[]>([]);
  const [devices, setDevices] = useState<any[]>([]);
  const [selProject, setSelProject] = useState<number|null>(null);
  const [selBuilding, setSelBuilding] = useState<number|null>(null);
  const [planType, setPlanType] = useState<string>('startup');
  const [steps, setSteps] = useState<any[]>([]);
  const [form] = Form.useForm();

  useEffect(() => { api.get('/projects').then(r=>setProjects(r.data)); api.get('/buildings').then(r=>setAllBuildings(r.data)); }, []);

  useEffect(() => {
    if(selProject){setBuildings(allBuildings.filter((b:any)=>Number(b.project_id)===Number(selProject)));setSelBuilding(null);}
    else{setBuildings([]);setSelBuilding(null);setPlans([]);}
  }, [selProject]);
  useEffect(() => {
    if(selBuilding){setLoading(true);api.get('/startup-plans?building_id='+selBuilding).then(r=>{setPlans(r.data);setLoading(false);});api.get('/devices?building_id='+selBuilding).then(r=>setDevices(r.data));}
    else{setPlans([]);setDevices([]);}
  }, [selBuilding]);

  const addStep = () => { setSteps([...steps, { device_id: devices[0]?.id||0, device_name: devices[0]?.name||'', wait_seconds: 20, sort_order: steps.length+1 }]); };
  const removeStep = (idx: number) => { setSteps(steps.filter((_,i)=>i!==idx).map((s,i)=>({...s,sort_order:i+1}))); };
  const moveStep = (idx: number, dir: number) => {
    var ns = [...steps]; var ti = idx+dir; if(ti<0||ti>=ns.length) return;
    [ns[idx],ns[ti]] = [ns[ti],ns[idx]]; ns = ns.map((s,i)=>({...s,sort_order:i+1})); setSteps(ns);
  };

  const save = async () => {
    var v = form.getFieldsValue();
    if(!v.name||!selBuilding||steps.length===0){message.warning('请填写名称并添加至少一个步骤');return;}
    var p: any = { name: v.name, building_id: selBuilding, plan_type: planType,
      steps: steps.map((s,i)=>({device_id:s.device_id,sort_order:i+1,wait_seconds:s.wait_seconds,action:planType})) };
    if(editing){ await api.put('/startup-plans/'+editing.ID, p); message.success('更新成功'); }
    else { await api.post('/startup-plans', p); message.success('创建成功'); }
    setModalOpen(false); setEditing(null); setSteps([]); form.resetFields();
    if(selBuilding) api.get('/startup-plans?building_id='+selBuilding).then(r=>setPlans(r.data));
  };
  const del = async (id: number) => { await api.delete('/startup-plans/'+id); message.success('已删除');
    if(selBuilding) api.get('/startup-plans?building_id='+selBuilding).then(r=>setPlans(r.data)); };
  const execute = async (id: number) => { await api.post('/startup-plans/'+id+'/execute'); message.success('已执行'); };

  const cols = [
    { title: 'ID', dataIndex: 'ID', width: 40 },
    { title: '名称', dataIndex: 'Name', width: 130 },
    { title: '类型', dataIndex: 'PlanType', width: 70, render: (v:string)=><Tag color={v==='startup'?'green':'red'}>{v==='startup'?'启动':'停止'}</Tag> },
    { title: '步骤', dataIndex: 'Steps', width: 300, render: (v:any)=>{
      if(!v||!Array.isArray(v)) return '-';
      try{var s=typeof v==='string'?JSON.parse(v):v; return s.map((x:any,i:number)=><span key={i}>{x.device_name||x.device_id}{i<s.length-1?' → ':''}</span>)}
      catch(e){return '-'}
    }},
    { title: '操作', width: 150, render: (_:any,r:any)=>(<Space size="small">
      <a onClick={()=>{setEditing(r);form.setFieldsValue(r);setPlanType(r.PlanType||'startup');
        var s=typeof r.Steps==='string'?JSON.parse(r.Steps||'[]'):(r.Steps||[]);setSteps(s.map((x:any,i:number)=>({device_id:x.device_id,device_name:x.device_name,wait_seconds:x.wait_seconds||20,sort_order:i+1})));setModalOpen(true);}}>编辑</a>
      <a onClick={()=>execute(r.ID)}><PlayCircleOutlined/>执行</a>
      <Popconfirm title="确定?" onConfirm={()=>del(r.ID)}><a style={{color:'red'}}>删除</a></Popconfirm>
    </Space>)},
  ];

  return (
    <div>
      <div style={{display:'flex',justifyContent:'space-between',marginBottom:12}}><h2>一键启停</h2>
        <Button type="primary" icon={<PlusOutlined/>} disabled={!selBuilding} onClick={()=>{setEditing(null);setSteps([]);setPlanType('startup');form.resetFields();setModalOpen(true);}}>新增</Button></div>
      <div style={{display:'flex',gap:10,marginBottom:12,alignItems:'flex-end'}}>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>项目</div><Select style={{width:180}} placeholder="项目" allowClear value={selProject} onChange={v=>setSelProject(v?Number(v):null)} options={projects.map(p=>({value:p.id,label:p.name}))}/></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>楼宇</div><Select style={{width:180}} placeholder="楼宇" allowClear value={selBuilding} disabled={!selProject} onChange={v=>setSelBuilding(v?Number(v):null)} options={buildings.map(b=>({value:b.id,label:b.name}))}/></div>
        {selBuilding && <span style={{paddingBottom:2,color:'#006875',fontWeight:500}}>共{plans.length}个方案</span>}
      </div>
      <Table rowKey="ID" columns={cols} dataSource={plans} loading={loading} scroll={{x:850}} size="small"
        locale={{emptyText:selBuilding?'暂无方案':'请先选择项目和楼宇'}}/>
      <Modal title={editing?'编辑':'新增'} width={700} open={modalOpen} onOk={save} onCancel={()=>{setModalOpen(false);setSteps([]);}}>
        <Form form={form} layout="vertical" initialValues={{plan_type:'startup'}}>
          <Row gutter={16}>
            <Col span={12}><Form.Item name="name" label="方案名称" rules={[{required:true}]}><Input/></Form.Item></Col>
            <Col span={12}><Form.Item name="plan_type" label="类型"><Select value={planType} onChange={v=>setPlanType(v)} options={[{value:'startup',label:'启动'},{value:'shutdown',label:'停止'}]}/></Form.Item></Col>
          </Row>
          <div style={{marginBottom:8,display:'flex',justifyContent:'space-between',alignItems:'center'}}>
            <strong>执行步骤（{planType==='startup'?'启动':'停止'}顺序）</strong>
            <Button size="small" type="dashed" icon={<PlusOutlined/>} onClick={addStep}>添加步骤</Button>
          </div>
          <div style={{maxHeight:300,overflowY:'auto',border:'1px solid #f0f0f0',borderRadius:8,padding:8,marginBottom:16}}>
            {steps.length===0 && <div style={{color:'#999',textAlign:'center',padding:20}}>暂无步骤，点击"添加步骤"</div>}
            {steps.map((s,idx)=>(<Row key={idx} gutter={8} style={{marginBottom:8,alignItems:'center',background:'#fafafa',padding:'6px 8px',borderRadius:6}}>
              <Col flex="30px"><span style={{color:'#006875',fontWeight:600}}>{idx+1}</span></Col>
              <Col flex="auto"><Select style={{width:'100%'}} value={s.device_id} onChange={v=>{var ns=[...steps];ns[idx].device_id=v;ns[idx].device_name=devices.find(d=>d.id===v)?.name||'';setSteps(ns);}} options={devices.map(d=>({value:d.id,label:d.name}))}/></Col>
              <Col flex="90px"><InputNumber style={{width:'100%'}} addonAfter="秒" min={1} max={300} value={s.wait_seconds} onChange={v=>{var ns=[...steps];ns[idx].wait_seconds=v||20;setSteps(ns);}}/></Col>
              <Col flex="60px"><Space size={0}><Button size="small" type="text" icon={<ArrowUpOutlined/>} disabled={idx===0} onClick={()=>moveStep(idx,-1)}/><Button size="small" type="text" icon={<ArrowDownOutlined/>} disabled={idx===steps.length-1} onClick={()=>moveStep(idx,1)}/><Button size="small" type="text" danger icon={<DeleteOutlined/>} onClick={()=>removeStep(idx)}/></Space></Col>
            </Row>))}
          </div>
        </Form>
      </Modal>
    </div>
  );
}