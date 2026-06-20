import { useEffect, useState } from 'react';
import { Table, Select, DatePicker, Button, Tabs, Card, Space, Tag, message } from 'antd';
import { DownloadOutlined, SearchOutlined } from '@ant-design/icons';
import api from '../api/client';
import dayjs from 'dayjs';

export default function Logs() {
  const [activeTab, setActiveTab] = useState('telemetry');
  const [projects, setProjects] = useState<any[]>([]);
  const [buildings, setBuildings] = useState<any[]>([]);
  const [devices, setDevices] = useState<any[]>([]);
  const [allBuildings, setAllBuildings] = useState<any[]>([]);
  const [selProject, setSelProject] = useState<number|null>(null);
  const [selBuilding, setSelBuilding] = useState<number|null>(null);
  const [selDevice, setSelDevice] = useState<number|null>(null);
  const [dateRange, setDateRange] = useState<[any,any]>([dayjs().subtract(7,'day'), dayjs()]);
  const [interval, setInterval] = useState('raw');
  const [data, setData] = useState<any[]>([]);
  const [stats, setStats] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => { api.get('/projects').then(r=>setProjects(r.data)); api.get('/buildings').then(r=>setAllBuildings(r.data)); }, []);
  useEffect(() => {
    if(selProject){setBuildings(allBuildings.filter((b:any)=>Number(b.project_id)===Number(selProject)));setSelBuilding(null);setSelDevice(null);}
    else{setBuildings([]);setSelBuilding(null);setSelDevice(null);}
  }, [selProject]);
  useEffect(() => { if(selBuilding){api.get('/devices?building_id='+selBuilding).then(r=>setDevices(r.data));setSelDevice(null);}else{setDevices([]);setSelDevice(null);} }, [selBuilding]);

  const fetchData = async () => {
    setLoading(true);
    var params = '?start='+(dateRange[0]?dateRange[0].format('YYYY-MM-DD'):'')+'&end='+(dateRange[1]?dateRange[1].format('YYYY-MM-DD'):'');
    if(selDevice) params += '&device_id='+selDevice;
    if(interval&&interval!=='raw') params += '&interval='+interval;
    if(activeTab==='telemetry'){
      api.get('/logs/telemetry'+params).then(r=>{setData(r.data);setLoading(false);});
    }else if(activeTab==='controls'){
      api.get('/logs/controls'+params).then(r=>{setData(r.data);setLoading(false);});
    }else{
      api.get('/logs/stats'+params).then(r=>{setStats(r.data);setLoading(false);});
    }
  };

  const exportCSV = () => {
    var params = '?export=csv&start='+(dateRange[0]?dateRange[0].format('YYYY-MM-DD'):'')+'&end='+(dateRange[1]?dateRange[1].format('YYYY-MM-DD'):'');
    if(selDevice) params += '&device_id='+selDevice;
    if(interval&&interval!=='raw') params += '&interval='+interval;
    window.open('/api/v1/logs/'+(activeTab==='controls'?'controls':'telemetry')+params, '_blank');
  };

  useEffect(() => { fetchData(); }, [activeTab, interval]);

  const telemetryCols = interval!=='raw' ? [
    { title:'时间', dataIndex:'ts', width:160, render:(v:any)=>dayjs(v).format('YYYY-MM-DD HH:mm') },
    { title:'指标', dataIndex:'metric', width:120 },
    { title:'平均值', dataIndex:'avg', width:100 },
    { title:'最大值', dataIndex:'max', width:100 },
    { title:'最小值', dataIndex:'min', width:100 },
    { title:'记录数', dataIndex:'count', width:80 },
  ] : [
    { title:'时间', dataIndex:'ts', width:180, render:(v:any)=>v?dayjs(v).format('YYYY-MM-DD HH:mm:ss'):'-' },
    { title:'指标', dataIndex:'metric', width:120 },
    { title:'值', dataIndex:'value', width:100 },
    { title:'单位', dataIndex:'unit', width:80 },
  ];

  const controlCols = [
    { title:'时间', dataIndex:'Ts', width:170, render:(v:any)=>v?dayjs(v).format('YYYY-MM-DD HH:mm:ss'):'-' },
    { title:'项目', dataIndex:'Proj', width:100 },
    { title:'楼宇', dataIndex:'Bld', width:100 },
    { title:'设备', dataIndex:'Dev', width:100 },
    { title:'属性', dataIndex:'Prop', width:100 },
    { title:'操作值', dataIndex:'Val', width:80 },
    { title:'操作人', dataIndex:'User', width:80 },
    { title:'备注', dataIndex:'Remark', width:120 },
  ];

  const statCols = [
    { title:'指标', dataIndex:'Metric', width:150 },
    { title:'记录数', dataIndex:'Count', width:80 },
    { title:'平均值', dataIndex:'Avg', width:100, render:(v:any)=>v?.toFixed(2) },
    { title:'合计', dataIndex:'Sum', width:120, render:(v:any)=>v?.toFixed(2) },
    { title:'最大值', dataIndex:'Max', width:100, render:(v:any)=>v?.toFixed(2) },
    { title:'最小值', dataIndex:'Min', width:100, render:(v:any)=>v?.toFixed(2) },
  ];

  return (
    <div>
      <h2 style={{marginBottom:12}}>日志管理</h2>
      <div style={{display:'flex',gap:10,marginBottom:12,alignItems:'flex-end',flexWrap:'wrap'}}>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>项目</div><Select style={{width:150}} placeholder="项目" allowClear value={selProject} onChange={v=>setSelProject(v?Number(v):null)} options={projects.map(p=>({value:p.id,label:p.name}))}/></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>楼宇</div><Select style={{width:160}} placeholder="楼宇" allowClear value={selBuilding} disabled={!selProject} onChange={v=>setSelBuilding(v?Number(v):null)} options={buildings.map(b=>({value:b.id,label:b.name}))}/></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>设备</div><Select style={{width:150}} placeholder="设备" allowClear value={selDevice} disabled={!selBuilding} onChange={v=>setSelDevice(v?Number(v):null)} options={devices.map(d=>({value:d.id,label:d.name}))}/></div>
        <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>日期范围</div><DatePicker.RangePicker size="middle" value={dateRange as any} onChange={(v:any)=>setDateRange(v||[dayjs().subtract(7,'day'),dayjs()])}/></div>
        {activeTab==='telemetry' && <div><div style={{marginBottom:2,color:'#666',fontSize:11}}>聚合</div><Select style={{width:80}} value={interval} onChange={v=>setInterval(v)} options={[{value:'raw',label:'原始'},{value:'hour',label:'小时'},{value:'day',label:'天'},{value:'month',label:'月'},{value:'year',label:'年'}]}/></div>}
        <Button type="primary" icon={<SearchOutlined/>} onClick={fetchData}>查询</Button>
        {activeTab!=='stats' && <Button icon={<DownloadOutlined/>} onClick={exportCSV}>导出CSV</Button>}
      </div>
      <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
        { key:'telemetry', label:'设备数据', children:<Table rowKey={(r:any,i:number)=>i} columns={telemetryCols} dataSource={data} loading={loading} scroll={{x:700}} size="small"/> },
        { key:'controls', label:'操作日志', children:<Table rowKey={(r:any,i:number)=>i} columns={controlCols} dataSource={data} loading={loading} scroll={{x:1000}} size="small"/> },
        { key:'stats', label:'数据统计', children:<Table rowKey="Metric" columns={statCols} dataSource={stats} loading={loading} size="small"/> },
      ]}/>
    </div>
  );
}