import { useEffect, useRef, useState } from 'react';
import axios from 'axios';
import Login from './Login';
import './index.css';

export default function Dashboard() {
  const [loggedIn, setLoggedIn] = useState(false);
  const [time, setTime] = useState('');
  const [cfg, setCfg] = useState<any>({});
  const [devices, setDevices] = useState<any[]>([]);
  const [alarms, setAlarms] = useState<any[]>([]);
  const [weather, setWeather] = useState({ temp: '28.5', hum: '65', desc: '晴' });

  useEffect(() => { if (localStorage.getItem('token')) setLoggedIn(true); }, []);

  useEffect(() => {
    if (!loggedIn) return;
    setInterval(() => setTime(new Date().toLocaleString('zh-CN')), 1000);
    fetchData();
    const i = setInterval(fetchData, 30000);
    return () => clearInterval(i);
  }, [loggedIn]);

  const fetchData = async () => {
    try {
      const h = { headers: { Authorization: 'Bearer ' + localStorage.getItem('token') } };
      const [d, a, cfgR] = await Promise.all([
        axios.get('/api/v1/devices?building_id=5', h),
        axios.get('/api/v1/alarm-logs', h),
        axios.get('/api/v1/dashboard', h),
      ]);
      setDevices(d.data); setAlarms(a.data.slice(0, 8)); setCfg(cfgR.data);
    } catch (e) { console.error(e); }
  };

  if (!loggedIn) return <Login onLogin={() => setLoggedIn(true)} />;

  // Group devices by type for topology
  const byType = (t: string) => devices.filter(d => d.device_type === t);
  const hosts = byType('主机'); const coolPumps = byType('冷却泵');
  const chillPumps = byType('冷冻泵'); const towers = byType('冷却塔');
  const valves = byType('阀门'); const secPumps = byType('二次泵');
  const online = devices.filter(d => d.online_status === '在线').length;

  return (
    <div className="dashboard">
      {/* Header */}
      <header className="topo-header">
        <div className="h-left"><span className="h-company">深圳市高海拔科技有限公司</span></div>
        <div className="h-center"><span className="h-title">熊猫智控 XMECO 多智能体能效节能系统</span></div>
        <div className="h-right"><span className="clock">{time}</span></div>
      </header>

      <div className="topo-content">
        {/* LEFT PANEL */}
        <div className="topo-panel left-panel">
          <div className="p-title">🌤️ 当地天气</div>
          <div className="weather-box">
            <div className="w-icon">☀️</div>
            <div className="w-temp">{weather.temp}°C</div>
            <div className="w-desc">{weather.desc} | 湿度 {weather.hum}%</div>
          </div>

          <div className="p-title" style={{ marginTop: 16 }}>⚠️ 最近告警</div>
          <div className="alarm-mini-list">
            {alarms.map((a: any, i: number) => (
              <div key={i} className={`alarm-row lvl-${a.Level||'info'}`}>
                <span className="a-time">{a.Ts?.slice(11, 16)}</span>
                <span className="a-dev">{a.DevName || '-'}</span>
                <span className="a-msg">{a.Msg || a.alarm_type}</span>
              </div>
            ))}
            {alarms.length === 0 && <div className="no-data">暂无告警</div>}
          </div>
        </div>

        {/* CENTER - Topology */}
        <div className="topo-panel center-panel">
          <div className="p-title">🏭 中央空调水冷系统拓扑</div>
          <div className="topology">
            {/* Cooling Tower */}
            <div className="topo-group tower-group">
              <div className="topo-label">冷却塔</div>
              <div className="topo-devices">
                {towers.map(d => (<div key={d.id} className={`topo-device ${d.online_status==='在线'?'online':'offline'}`} title={d.name}><span>{d.name.replace('冷却塔','CT')}</span></div>))}
              </div>
            </div>

            {/* Flow lines and arrows */}
            <div className="flow-line flow-tower-to-coolpump">
              <div className="flow-arrow">→</div>
            </div>

            {/* Cooling Pump */}
            <div className="topo-group coolpump-group">
              <div className="topo-label">冷却泵</div>
              <div className="topo-devices">
                {coolPumps.map(d => (<div key={d.id} className={`topo-device ${d.online_status==='在线'?'online':'offline'}`} title={d.name}><span>{d.name.replace('冷却泵','CP')}</span></div>))}
              </div>
            </div>

            <div className="flow-line flow-coolpump-to-chiller">
              <div className="flow-arrow">→</div>
            </div>

            {/* Chiller (Host) */}
            <div className="topo-group chiller-group">
              <div className="topo-label">冷水机组</div>
              <div className="topo-devices big">
                {hosts.map(d => (<div key={d.id} className={`topo-device big-device ${d.online_status==='在线'?'online':'offline'}`} title={d.name}><span>{d.name.replace('约克主机','YK')}</span></div>))}
              </div>
            </div>

            <div className="flow-line flow-chiller-to-chillpump">
              <div className="flow-arrow">→</div>
            </div>

            {/* Chilled Pump */}
            <div className="topo-group chillpump-group">
              <div className="topo-label">冷冻泵</div>
              <div className="topo-devices">
                {chillPumps.map(d => (<div key={d.id} className={`topo-device ${d.online_status==='在线'?'online':'offline'}`} title={d.name}><span>{d.name.replace('冷冻泵','FP')}</span></div>))}
              </div>
            </div>

            <div className="flow-line flow-chillpump-to-end">
              <div className="flow-arrow">→</div>
            </div>

            {/* Terminal */}
            <div className="topo-group terminal-group">
              <div className="topo-label">末端设备</div>
              <div className="topo-devices small">
                {valves.map(d => (<div key={d.id} className={`topo-device tiny ${d.online_status==='在线'?'online':'offline'}`} title={d.name}><span>阀</span></div>))}
                {secPumps.map(d => (<div key={d.id} className={`topo-device tiny ${d.online_status==='在线'?'online':'offline'}`} title={d.name}><span>SP</span></div>))}
              </div>
            </div>

            {/* Return flow */}
            <div className="flow-return">← 回水循环 ←</div>

            {/* Status legend */}
            <div className="topo-legend">
              <span className="leg-item"><span className="leg-dot on" /> 在线 {online}</span>
              <span className="leg-item"><span className="leg-dot off" /> 离线 {devices.length - online}</span>
              <span className="leg-item">共 {devices.length} 台</span>
            </div>
          </div>
        </div>

        {/* RIGHT PANEL */}
        <div className="topo-panel right-panel">
          <div className="p-title">⚡ 节能指标</div>
          <div className="energy-card">
            <div className="ec-big">{cfg.power_saved || '1,245'}<span> 度</span></div>
            <div className="ec-label">累计节电量</div>
            <div className="ec-bar"><div className="ec-fill" style={{ width: '78%' }} /></div>
          </div>
          <div className="energy-card">
            <div className="ec-big green">{cfg.carbon_saved || '986'}<span> 吨</span></div>
            <div className="ec-label">累计节碳量</div>
            <div className="ec-bar"><div className="ec-fill green" style={{ width: '65%' }} /></div>
          </div>
          <div className="energy-card">
            <div className="ec-big cyan">{cfg.running_days || '2000'}<span> 天</span></div>
            <div className="ec-label">安全运行</div>
            <div className="ec-bar"><div className="ec-fill cyan" style={{ width: '92%' }} /></div>
          </div>

          <div className="p-title" style={{ marginTop: 16 }}>📊 设备统计</div>
          <div className="stat-mini-grid">
            <div className="stat-mini"><div className="sm-val">{hosts.length}</div><div className="sm-lbl">主机</div></div>
            <div className="stat-mini"><div className="sm-val">{coolPumps.length}</div><div className="sm-lbl">冷却泵</div></div>
            <div className="stat-mini"><div className="sm-val">{chillPumps.length}</div><div className="sm-lbl">冷冻泵</div></div>
            <div className="stat-mini"><div className="sm-val">{towers.length}</div><div className="sm-lbl">冷却塔</div></div>
            <div className="stat-mini"><div className="sm-val">{valves.length}</div><div className="sm-lbl">阀门</div></div>
            <div className="stat-mini"><div className="sm-val">{secPumps.length}</div><div className="sm-lbl">二次泵</div></div>
          </div>

          <div className="p-title" style={{ marginTop: 12 }}>📈 今日概况</div>
          <div className="stat-mini-grid">
            <div className="stat-mini"><div className="sm-val green">{online}</div><div className="sm-lbl">在线</div></div>
            <div className="stat-mini"><div className="sm-val red">{alarms.length}</div><div className="sm-lbl">告警</div></div>
            <div className="stat-mini"><div className="sm-val">{cfg.today_alarms || '2000'}</div><div className="sm-lbl">今日告警</div></div>
          </div>
        </div>
      </div>
    </div>
  );
}