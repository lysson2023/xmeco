import { useEffect, useState, useRef } from 'react';
import { Table, Button, Modal, Form, Input, Select, Space, message, Popconfirm, Row, Col, InputNumber, Tag, Tabs, Switch, TimePicker } from 'antd';
import { PlusOutlined, DeleteOutlined, ArrowUpOutlined, ArrowDownOutlined, PlayCircleOutlined, ClockCircleOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import api from '../api/client';
import dayjs from 'dayjs';

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
  const [searchParams, setSearchParams] = useSearchParams();
  const mounted = useRef(false);
  const restoring = useRef(false);

  // ---- Scheduled Tasks ----
  const [tasks, setTasks] = useState<any[]>([]);
  const [taskModalOpen, setTaskModalOpen] = useState(false);
  const [taskEditing, setTaskEditing] = useState<any>(null);
  const [taskSelProject, setTaskSelProject] = useState<number|null>(null);
  const [taskSelBuilding, setTaskSelBuilding] = useState<number|null>(null);
  const [taskBuildings, setTaskBuildings] = useState<any[]>([]);
  const [taskDevices, setTaskDevices] = useState<any[]>([]);
  const [taskForm] = Form.useForm();

  useEffect(() => { api.get('/projects').then(r=>setProjects(r.data)); api.get('/buildings').then(r => {
    setAllBuildings(r.data);
    const bid = searchParams.get('building_id');
    if (bid) {
      const bld = r.data.find((b: any) => Number(b.id) === Number(bid));
      if (bld) {
        restoring.current = true;
        setSelProject(Number(bld.project_id));
        setSelBuilding(Number(bid));
      }
    }
  }); }, []);

  // ---- Startup Plans cascade ----
  useEffect(() => {
    if(selProject){setBuildings(allBuildings.filter((b:any)=>Number(b.project_id)===Number(selProject))); if (!restoring.current) setSelBuilding(null);}
    else{setBuildings([]);setSelBuilding(null);setPlans([]);}
  }, [selProject]);
  useEffect(() => {
    if(selBuilding){setLoading(true);api.get('/startup-plans?building_id='+selBuilding).then(r=>{setPlans(r.data);setLoading(false); restoring.current = false;}).catch(()=>setLoading(false));api.get('/devices?building_id='+selBuilding).then(r=>setDevices(r.data));}
    else{setPlans([]);setDevices([]);}
  }, [selBuilding]);
  useEffect(() => {
    if (selBuilding) setSearchParams({ building_id: String(selBuilding) });
    else if (mounted.current) setSearchParams({});
    mounted.current = true;
  }, [selBuilding]);

  // ---- Scheduled Tasks cascade ----
  useEffect(() => {
    if(taskSelProject){setTaskBuildings(allBuildings.filter((b:any)=>Number(b.project_id)===Number(taskSelProject))); setTaskSelBuilding(null);}
    else{setTaskBuildings([]);setTaskSelBuilding(null);setTasks([]);}
  }, [taskSelProject]);
  useEffect(() => {
    if(taskSelBuilding){api.get('/scheduled-tasks?building_id='+taskSelBuilding).then(r=>setTasks(r.data));api.get('/devices?building_id='+taskSelBuilding).then(r=>setTaskDevices(r.data));}
    else{setTasks([]);setTaskDevices([]);}
  }, [taskSelBuilding]);

  // ---- Startup helpers ----
  const getBldName = (bid: number) => allBuildings.find((b:any)=>Number(b.id)===bid)?.name||'-';
  const getProjName = (bid: number) => {
    const bld = allBuildings.find((b:any)=>Number(b.id)===bid);
    if (!bld) return '-';
    return projects.find((p:any)=>Number(p.id)===Number(bld.project_id))?.name||'-';
  };
  const addStep = () => { setSteps([...steps, { device_id: devices[0]?.id||0, device_name: devices[0]?.name||'', wait_seconds: 20, retry_count: 1, sort_order: steps.length+1 }]); };
  const removeStep = (idx: number) => { setSteps(steps.filter((_,i)=>i!==idx).map((s,i)=>({...s,sort_order:i+1}))); };
  const moveStep = (idx: number, dir: number) => {
    let ns = [...steps]; const ti = idx+dir; if(ti<0||ti>=ns.length) return;
    [ns[idx],ns[ti]] = [ns[ti],ns[idx]]; ns = ns.map((s,i)=>({...s,sort_order:i+1})); setSteps(ns);
  };

  const save = async () => {
    try {
      const v = form.getFieldsValue();
      if(!v.name||!selBuilding||steps.length===0){message.warning('请填写名称并添加至少一个步骤');return;}
      const p: any = { name: v.name, building_id: selBuilding, plan_type: planType,
        steps: steps.map((s,i)=>({device_id:s.device_id,sort_order:i+1,wait_seconds:s.wait_seconds,retry_count:s.retry_count||1,action:planType})) };
      if(editing){ await api.put('/startup-plans/'+editing.id, p); message.success('更新成功'); }
      else { await api.post('/startup-plans', p); message.success('创建成功'); }
      setModalOpen(false); setEditing(null); setSteps([]); form.resetFields();
      if(selBuilding) api.get('/startup-plans?building_id='+selBuilding).then(r=>setPlans(r.data));
    } catch { message.error('保存失败'); }
  };
  const del = async (id: number) => {
    try {
      await api.delete('/startup-plans/'+id); message.success('已删除');
      if(selBuilding) api.get('/startup-plans?building_id='+selBuilding).then(r=>setPlans(r.data));
    } catch { message.error('删除失败'); }
  };
  const execute = async (id: number) => {
    try {
      await api.post('/startup-plans/'+id+'/execute'); message.success('已执行');
    } catch { message.error('执行失败'); }
  };

  // ---- Scheduled Task helpers ----
  const taskSave = async () => {
    try {
      const v = taskForm.getFieldsValue();
      if(!v.name||!taskSelBuilding||!v.device_id||!v.schedule_time){message.warning('请填写必填项');return;}
      const timeStr = v.schedule_time.format('HH:mm:ss');
      const payload: any = {
        name: v.name, building_id: taskSelBuilding, device_id: v.device_id,
        action_type: v.action_type||'startup', target_value: v.target_value||null,
        schedule_type: v.schedule_type||'once', schedule_time: timeStr,
        days_of_week: v.days_of_week||null, enabled: v.enabled!==false,
      };
      if(taskEditing){ await api.put('/scheduled-tasks/'+taskEditing.id, payload); message.success('已更新'); }
      else { await api.post('/scheduled-tasks', payload); message.success('已创建'); }
      setTaskModalOpen(false); setTaskEditing(null); taskForm.resetFields();
      if(taskSelBuilding) api.get('/scheduled-tasks?building_id='+taskSelBuilding).then(r=>setTasks(r.data));
    } catch { message.error('保存失败'); }
  };
  const taskDel = async (id: number) => {
    try {
      await api.delete('/scheduled-tasks/'+id); message.success('已删除');
      if(taskSelBuilding) api.get('/scheduled-tasks?building_id='+taskSelBuilding).then(r=>setTasks(r.data));
    } catch { message.error('删除失败'); }
  };

  const actionLabel = (a: string) => { const m: any = {startup:'开机',shutdown:'关机',set_value:'设值',mode_change:'切换模式'}; return m[a]||a; };
  const actionColor = (a: string) => { const m: any = {startup:'green',shutdown:'red',set_value:'blue',mode_change:'orange'}; return m[a]||'default'; };
  const scheduleLabel = (s: string) => { const m: any = {once:'单次',daily:'每天',weekly:'每周'}; return m[s]||s; };

  const startupCols = [
    { title: 'ID', dataIndex: 'id', width: 40 },
    { title: '名称', dataIndex: 'name', width: 130 },
    { title: '所属项目', dataIndex: 'building_id', width: 100, render: (v:number) => getProjName(v) },
    { title: '所属楼宇', dataIndex: 'building_id', width: 100, render: (v:number) => getBldName(v) },
    { title: '类型', dataIndex: 'plan_type', width: 70, render: (v:string)=><Tag color={v==='startup'?'green':'red'}>{v==='startup'?'启动':'停止'}</Tag> },
    { title: '步骤', dataIndex: 'steps', width: 300, render: (v:any)=>{
      if(!v||!Array.isArray(v)) return '-';
      try{const s=typeof v==='string'?JSON.parse(v):v; return s.map((x:any,i:number)=><span key={i}>{x.device_name||x.device_id}{i<s.length-1?' → ':''}</span>)}
      catch{return '-'}
    }},
    { title: '操作', width: 150, render: (_:any,r:any)=>(<Space size="small">
      <a onClick={()=>{setEditing(r);form.setFieldsValue(r);setPlanType(r.plan_type||'startup');
        const s=typeof r.steps==='string'?JSON.parse(r.steps||'[]'):(r.steps||[]);setSteps(s.map((x:any,i:number)=>({device_id:x.device_id,device_name:x.device_name,wait_seconds:x.wait_seconds||20,retry_count:x.retry_count||1,sort_order:i+1})));setModalOpen(true);}}>编辑</a>
      <a onClick={()=>execute(r.id)}><PlayCircleOutlined/>执行</a>
      <Popconfirm title="确定?" onConfirm={()=>del(r.id)}><a style={{color:'red'}}>删除</a></Popconfirm>
    </Space>)},
  ];

  const taskCols = [
    { title: 'ID', dataIndex: 'id', width: 40 },
    { title: '名称', dataIndex: 'name', width: 120 },
    { title: '设备', dataIndex: 'device_name', width: 100 },
    { title: '动作', dataIndex: 'action_type', width: 80, render: (v:string)=><Tag color={actionColor(v)}>{actionLabel(v)}</Tag> },
    { title: '目标值', dataIndex: 'target_value', width: 80, render: (v:any)=>v||'-' },
    { title: '周期', dataIndex: 'schedule_type', width: 70, render: (v:string)=>scheduleLabel(v) },
    { title: '执行时间', dataIndex: 'schedule_time', width: 90 },
    { title: '启用', dataIndex: 'enabled', width: 60, render: (v:boolean, r:any) =>
      <Switch size="small" checked={v} onChange={async (c)=>{
        try {
          await api.put('/scheduled-tasks/'+r.id, { enabled: c });
          setTasks(tasks.map(t=>t.id===r.id?{...t,enabled:c}:t));
        } catch { message.error('操作失败'); }
      }} />
    },
    { title: '上次结果', dataIndex: 'last_result', width: 80, render: (v:any)=>
      v ? <Tag color={v==='success'?'green':'red'}>{v==='success'?'成功':'失败'}</Tag> : '-'
    },
    { title: '操作', width: 100, render: (_:any,r:any)=>(<Space size="small">
      <a onClick={()=>{
        setTaskEditing(r); taskForm.setFieldsValue({...r, schedule_time: r.schedule_time?dayjs(r.schedule_time,'HH:mm:ss'):null});
        setTaskSelBuilding(r.building_id); setTaskModalOpen(true);
      }}>编辑</a>
      <Popconfirm title="确定?" onConfirm={()=>taskDel(r.id)}><a style={{color:'red'}}>删除</a></Popconfirm>
    </Space>)},
  ];

  // ---- Tab: 一键启停 ----
  const startupTab = (
    <div>
      <div style={{display:'flex',justifyContent:'space-between',marginBottom:12}}>
        <span></span>
        <Button type="primary" icon={<PlusOutlined/>} disabled={!selBuilding} onClick={()=>{setEditing(null);setSteps([]);setPlanType('startup');form.resetFields();setModalOpen(true);}}>新增</Button>
      </div>
      <div style={{display:'flex',gap:10,marginBottom:12,alignItems:'flex-end'}}>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>项目</div><Select style={{width:180}} placeholder="项目" allowClear value={selProject} onChange={v=>setSelProject(v?Number(v):null)} options={projects.map(p=>({value:p.id,label:p.name}))}/></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>楼宇</div><Select style={{width:180}} placeholder="楼宇" allowClear value={selBuilding} disabled={!selProject} onChange={v=>setSelBuilding(v?Number(v):null)} options={buildings.map(b=>({value:b.id,label:b.name}))}/></div>
        {selBuilding && <span style={{paddingBottom:2,color:'#006875',fontWeight:500}}>共{plans.length}个方案</span>}
      </div>
      <Table rowKey="id" columns={startupCols} dataSource={plans} loading={loading} scroll={{x:950}} size="small"
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
            {steps.map((s,idx)=>(<Row key={s.device_id+'-'+idx} gutter={8} style={{marginBottom:8,alignItems:'center',background:'#fafafa',padding:'6px 8px',borderRadius:6}}>
              <Col flex="30px"><span style={{color:'#006875',fontWeight:600}}>{idx+1}</span></Col>
              <Col flex="auto"><Select style={{width:'100%'}} value={s.device_id} onChange={v=>{const ns=[...steps];ns[idx].device_id=v;ns[idx].device_name=devices.find(d=>d.id===v)?.name||'';setSteps(ns);}} options={devices.map(d=>({value:d.id,label:d.name}))}/></Col>
              <Col flex="90px"><InputNumber style={{width:'100%'}} addonAfter="秒" min={1} max={300} value={s.wait_seconds} onChange={v=>{const ns=[...steps];ns[idx].wait_seconds=v||20;setSteps(ns);}}/></Col>
              <Col flex="60px"><Space size={0}><Button size="small" type="text" icon={<ArrowUpOutlined/>} disabled={idx===0} onClick={()=>moveStep(idx,-1)}/><Button size="small" type="text" icon={<ArrowDownOutlined/>} disabled={idx===steps.length-1} onClick={()=>moveStep(idx,1)}/><Button size="small" type="text" danger icon={<DeleteOutlined/>} onClick={()=>removeStep(idx)}/></Space></Col>
            </Row>))}
          </div>
        </Form>
      </Modal>
    </div>
  );

  // ---- Tab: 定时任务 ----
  const taskTab = (
    <div>
      <div style={{display:'flex',justifyContent:'space-between',marginBottom:12}}>
        <span></span>
        <Button type="primary" icon={<PlusOutlined/>} disabled={!taskSelBuilding}
          onClick={()=>{setTaskEditing(null);taskForm.resetFields();setTaskModalOpen(true);}}>新增</Button>
      </div>
      <div style={{display:'flex',gap:10,marginBottom:12,alignItems:'flex-end'}}>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>项目</div><Select style={{width:180}} placeholder="项目" allowClear value={taskSelProject} onChange={v=>setTaskSelProject(v?Number(v):null)} options={projects.map(p=>({value:p.id,label:p.name}))}/></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>楼宇</div><Select style={{width:180}} placeholder="楼宇" allowClear value={taskSelBuilding} disabled={!taskSelProject} onChange={v=>setTaskSelBuilding(v?Number(v):null)} options={taskBuildings.map(b=>({value:b.id,label:b.name}))}/></div>
        {taskSelBuilding && <span style={{paddingBottom:2,color:'#006875',fontWeight:500}}>共{tasks.length}个任务</span>}
      </div>
      <Table rowKey="id" columns={taskCols} dataSource={tasks} scroll={{x:900}} size="small"
        locale={{emptyText:taskSelBuilding?'暂无任务':'请先选择项目和楼宇'}}/>
      <Modal title={taskEditing?'编辑任务':'新增任务'} width={550} open={taskModalOpen} onOk={taskForm.submit} onCancel={()=>{setTaskModalOpen(false);setTaskEditing(null);}}>
        <Form form={taskForm} layout="vertical" onFinish={taskSave} initialValues={{schedule_type:'daily',action_type:'startup',enabled:true}}>
          <Form.Item name="name" label="任务名称" rules={[{required:true}]}><Input placeholder="例如：早8点开机"/></Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="device_id" label="选择设备" rules={[{required:true}]}>
                <Select placeholder="选择设备" options={taskDevices.map((d:any)=>({value:d.id,label:d.name}))}/>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="action_type" label="动作类型" rules={[{required:true}]}>
                <Select options={[
                  {value:'startup',label:'开机'},{value:'shutdown',label:'关机'},
                  {value:'set_value',label:'设定数值'},{value:'mode_change',label:'切换模式'},
                ]}/>
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="target_value" label="目标值（设定数值/切换模式时填写）">
            <Input placeholder="例如：7°C 或 制冷模式"/>
          </Form.Item>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="schedule_type" label="执行周期">
                <Select options={[
                  {value:'once',label:'单次'},{value:'daily',label:'每天'},{value:'weekly',label:'每周'},
                ]}/>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="schedule_time" label="执行时间" rules={[{required:true}]}>
                <TimePicker format="HH:mm" style={{width:'100%'}}/>
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="days_of_week" label="每周哪几天（1-7）" extra="逗号分隔，1=周一">
                <Input placeholder="1,2,3,4,5"/>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="enabled" label="启用" valuePropName="checked">
                <Switch/>
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>
    </div>
  );

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}><ClockCircleOutlined style={{ marginRight: 8, color: '#006875' }} />启停配置</h2>
      <Tabs defaultActiveKey="startup" size="large" items={[
        { key: 'startup', label: <span><PlayCircleOutlined /> 一键启停</span>, children: startupTab },
        { key: 'scheduled', label: <span><ClockCircleOutlined /> 定时任务</span>, children: taskTab },
      ]} />
    </div>
  );
}
