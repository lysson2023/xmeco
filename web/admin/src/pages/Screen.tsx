import { useEffect, useState, useCallback, useRef } from 'react';
import { Select, InputNumber, Button, Spin, Tag, message, Modal, Switch } from 'antd';
import {
  LogoutOutlined, ThunderboltOutlined,
  EnvironmentOutlined, ClockCircleOutlined, AlertOutlined,
  DashboardOutlined, DatabaseOutlined, ToolOutlined,
  ScheduleOutlined, RocketOutlined, BulbOutlined,
  PlayCircleOutlined,
} from '@ant-design/icons';
import { api, setAuth, isTokenExpired } from '../api/screenClient';
import DataCenter from './DataCenter';
import MaintenanceCenter from './MaintenanceCenter';
import ScreenTaskCenter from './ScreenTaskCenter';
import ScreenDecisionCenter from './ScreenDecisionCenter';
import ScreenEnergyCenter from './ScreenEnergyCenter';
import ErrorBoundary from '../components/ErrorBoundary';
import { TopoRow, TopoCol } from '../components/TopoDevice';
import { TOPO_ORDER, TOPO_COLORS, QUICK_MODES } from '../utils/constants';
import './Login.css';
import './Screen.css';

// ---- Global keyframes for fault flashing (inject once, guarded by ID) ----
if (!document.getElementById('screen-fault-pulse-style')) {
  const styleSheet = document.createElement('style');
  styleSheet.id = 'screen-fault-pulse-style';
  styleSheet.textContent = `@keyframes faultPulse { 0%,100% { box-shadow: 0 0 4px #ff4d4f; } 50% { box-shadow: 0 0 14px #ff4d4f; } }`;
  document.head.appendChild(styleSheet);
}

// ---- Module-level constants (avoid re-creation on each render) ----
const TABS = [
  { key: 'monitor', icon: <DashboardOutlined />, label: '监控中心' },
  { key: 'data', icon: <DatabaseOutlined />, label: '数据中心' },
  { key: 'maintain', icon: <ToolOutlined />, label: '维保中心' },
  { key: 'task', icon: <ScheduleOutlined />, label: '任务中心' },
  { key: 'decision', icon: <RocketOutlined />, label: '决策中心' },
  { key: 'energy', icon: <BulbOutlined />, label: '能耗中心' },
];

export default function Screen() {
  // ---- Auth ----
  const [uname, setUname] = useState('');
  const [pwd, setPwd] = useState('');
  const [loginLoading, setLoginLoading] = useState(false);
  const [loggedIn, setLoggedIn] = useState(false);

  // ---- Data ----
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<any>({});
  const [pid, setPid] = useState<number>(0);
  const [bid, setBid] = useState<number>(0);
  const [tab, setTab] = useState('monitor');
  const [devModal, setDevModal] = useState<{ open: boolean; dev: any }>({ open: false, dev: null });
  const [devProps, setDevProps] = useState<any[]>([]);
  const [propsLoading, setPropsLoading] = useState(false);

  // ---- Refs for race condition prevention ----
  const fetchSeqRef = useRef(0);  // 请求序号，防止旧请求覆盖新数据
  const openDevSeqRef = useRef(0); // 设备属性加载序号
  const mountedRef = useRef(true); // 组件挂载标志，防止卸载后 setState

  // ---- Login ----
  const doLogin = async () => {
    if (!uname || !pwd) { message.warning('请输入用户名和密码'); return; }
    setLoginLoading(true);
    try {
      const r = await api.post('/auth/login', { username: uname, password: pwd });
      const t = r.data.token;
      setAuth(t);
      localStorage.setItem('screen_token', t);
      localStorage.setItem('screen_user', uname);
      setTab('monitor'); // 登录后确保从监控中心开始
      setLoggedIn(true);
    } catch { message.error('用户名或密码错误'); }
    finally { setLoginLoading(false); }
  };

  // ---- Init: check for saved token (validate expiry before trusting it) ----
  useEffect(() => {
    mountedRef.current = true;
    const saved = localStorage.getItem('screen_token');
    if (saved && !isTokenExpired(saved)) {
      setAuth(saved);
      setUname(localStorage.getItem('screen_user') || '');
      setLoggedIn(true);
    } else if (saved) {
      localStorage.removeItem('screen_token');
      localStorage.removeItem('screen_user');
    }
    // Listen for 401 auto-logout (idempotent guard)
    const handler = () => { if (mountedRef.current) setLoggedIn(prev => prev ? false : prev); };
    window.addEventListener('screen-auth-expired', handler);
    return () => {
      mountedRef.current = false;
      window.removeEventListener('screen-auth-expired', handler);
    };
  }, []);

  // ---- Fetch screen data (race-safe via request sequence number) ----
  const fetchData = useCallback(async () => {
    if (!loggedIn) return;
    setLoading(true);
    const seq = ++fetchSeqRef.current; // 本次请求的序号
    try {
      const p: any = {}; if (pid) p.project_id = pid; if (bid) p.building_id = bid;
      const r = await api.get('/screen/data', { params: p });
      // 仅当本次请求仍是最新请求时才更新数据，防止旧请求覆盖新数据
      if (seq !== fetchSeqRef.current || !mountedRef.current) return;
      setData(r.data);
    } catch (err: any) {
      if (seq !== fetchSeqRef.current || !mountedRef.current) return;
      if (err?.response?.status !== 401) {
        message.error('数据加载失败');
      }
    }
    finally {
      if (seq === fetchSeqRef.current && mountedRef.current) setLoading(false);
    }
  }, [loggedIn, pid, bid]);

  useEffect(() => { if (loggedIn) fetchData(); }, [fetchData]);
  useEffect(() => {
    if (!loggedIn) return;
    const t = setInterval(fetchData, 5000);
    return () => clearInterval(t);
  }, [fetchData, loggedIn]);

  // ---- Auto-select first project when data arrives (separated from fetch to avoid cascade) ----
  useEffect(() => {
    if (!pid && data.projects?.length) {
      setPid(data.projects[0].id);
    }
  }, [data.projects, pid]);

  // ---- Logout ----
  const logout = () => {
    localStorage.removeItem('screen_token');
    localStorage.removeItem('screen_user');
    setTab('monitor');
    setLoggedIn(false);
  };

  // ---- Device click → load properties ----
  const openDevice = async (dev: any) => {
    setDevModal({ open: true, dev });
    setPropsLoading(true);
    const seq = ++openDevSeqRef.current;
    try {
      const r = await api.get('/properties', { params: { device_id: dev.id } });
      if (seq !== openDevSeqRef.current || !mountedRef.current) return;
      setDevProps(r.data || []);
    } catch (err: any) {
      if (seq !== openDevSeqRef.current || !mountedRef.current) return;
      if (err?.response?.status !== 401) {
        setDevProps([]);
      }
    } finally {
      if (seq === openDevSeqRef.current && mountedRef.current) setPropsLoading(false);
    }
  };

  // ---- Device control ----
  const doControl = async (devId: number, action: string, targetVal?: string) => {
    try {
      await api.post(`/devices/${devId}/control`, { action, target_value: targetVal || '' });
      message.success('指令已发送');
    } catch (err: any) {
      // 401 已由拦截器处理，不重复弹错误提示
      if (err?.response?.status !== 401) message.error('控制失败');
    }
  };

  // ---- Build topology groups ----
  const groups: Record<string, any[]> = {};
  (data.devices || []).forEach((d: any) => {
    const key = TOPO_ORDER.includes(d.type) ? d.type : '其他';
    if (!groups[key]) groups[key] = [];
    groups[key].push(d);
  });

  // ==================== LOGIN SCREEN ====================
  if (!loggedIn) {
    return (
      <div className="login-wrapper">
        <div className="bg-layer">
          <div className="bg-subgrid" />
          <div className="bg-grid" />
          <div className="fiber-line" style={{ top: '15%', animationDelay: '0s' }} />
          <div className="fiber-line" style={{ top: '45%', animationDelay: '2s', width: '200px', opacity: 0.4 }} />
          <div className="fiber-line" style={{ top: '75%', animationDelay: '1.5s' }} />
          <div className="fiber-line" style={{ top: '90%', animationDelay: '3s', width: '300px' }} />
          <div className="junction" style={{ top: '20%', left: '30%' }} />
          <div className="junction" style={{ top: '40%', left: '70%', animationDelay: '0.5s' }} />
          <div className="junction" style={{ top: '60%', left: '20%', animationDelay: '1.2s' }} />
          <div className="junction" style={{ top: '80%', left: '85%', animationDelay: '0.8s' }} />
          <div className="junction" style={{ top: '15%', left: '55%', animationDelay: '1.5s' }} />
          <div className="junction" style={{ top: '75%', left: '45%', animationDelay: '2s' }} />
          <div className="bg-glow" />
        </div>

        <div className="particles-layer">
          <div className="particle sm" style={{ top: '90%', left: '10%', '--tx': '100px', '--ty': '-900px' } as React.CSSProperties} />
          <div className="particle sm" style={{ top: '95%', left: '40%', '--tx': '-50px', '--ty': '-900px', animationDelay: '1s' } as React.CSSProperties} />
          <div className="particle" style={{ top: '85%', left: '70%', '--tx': '200px', '--ty': '-800px', animationDelay: '2s' } as React.CSSProperties} />
          <div className="particle md" style={{ top: '50%', left: '50%', '--tx': '300px', '--ty': '-200px', animationDelay: '0.5s' } as React.CSSProperties} />
          <div className="particle md" style={{ top: '20%', left: '80%', '--tx': '-400px', '--ty': '400px', animationDelay: '3s', opacity: 0.5 } as React.CSSProperties} />
          <div className="particle md" style={{ top: '60%', left: '20%', '--tx': '500px', '--ty': '100px' } as React.CSSProperties} />
          <div className="particle lg" style={{ top: '10%', left: '30%', '--tx': '100px', '--ty': '800px' } as React.CSSProperties} />
          <div className="particle lg" style={{ top: '80%', left: '60%', '--tx': '-200px', '--ty': '-700px', animationDelay: '5s' } as React.CSSProperties} />
        </div>

        <div className="central-hub">
          <div className="hub-core" />
          <div className="hub-ring ring-1" />
          <div className="hub-ring ring-2" />
        </div>

        <main className="login-card-wrapper">
          <div className="login-card">
            <div className="scan-line" />
            <div className="brand-area">
              <h1 className="brand-title">熊猫智控<span className="brand-sub">XMECO</span></h1>
              <p className="brand-tagline">智慧能效大屏系统</p>
            </div>
            <form className="login-form" onSubmit={e => { e.preventDefault(); doLogin(); }}>
              <div className="input-group">
                <label className="input-label">用户名</label>
                <div className="input-wrap">
                  <input className="login-input" type="text" placeholder="请输入用户名"
                    value={uname} onChange={e => setUname(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && doLogin()} />
                  <div className="input-underline" />
                  <span className="input-icon">👤</span>
                </div>
              </div>
              <div className="input-group">
                <label className="input-label">密码</label>
                <div className="input-wrap">
                  <input className="login-input" type="password" placeholder="••••••••••"
                    value={pwd} onChange={e => setPwd(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && doLogin()} />
                  <div className="input-underline" />
                  <span className="input-icon">🔒</span>
                </div>
              </div>
              <button className="login-btn" type="submit" disabled={loginLoading}>
                <span className="btn-text">{loginLoading ? '登录中…' : '登录'}</span>
                <div className="btn-shine" />
              </button>
            </form>
          </div>
        </main>

        <footer className="login-footer"><div>© 深圳市高海拔科技有限公司</div></footer>
      </div>
    );
  }

  // ==================== DASHBOARD ====================
  // 首次加载时显示全屏 Spin；后续轮询不显示（避免卸载子组件导致 state 丢失）
  const isFirstLoad = loading && !data.projects;
  if (isFirstLoad) {
    return <div style={{ textAlign: 'center', padding: '40vh 0', background: '#0b1515', minHeight: '100vh' }}><Spin size="large" /></div>;
  }

  return (
    <div className="screen-body" style={{ minHeight: '100vh' }}>
      {/* Row 1: Header */}
      <div className="screen-header" style={{ display: 'flex', alignItems: 'center', padding: '6px 20px' }}>
        <Select style={{ width: 140 }} placeholder="项目" value={pid || undefined}
          onChange={v => { setPid(v); setBid(0); }}
          options={(data.projects || []).map((p: any) => ({ value: p.id, label: p.name }))} />
        <Select style={{ width: 140, marginLeft: 8 }} placeholder="楼宇" value={bid || undefined} allowClear
          onChange={v => setBid(v || 0)}
          options={(data.buildings || []).map((b: any) => ({ value: b.id, label: b.name }))} disabled={!pid} />
        <div style={{ flex: 1, textAlign: 'center', fontSize: 16, fontWeight: 700, color: '#00daf3', letterSpacing: 1 }}>
          {(data.agent_name || '高海拔科技')}熊猫智控 XMECO 多智能体能效节能系统
        </div>
        <span style={{ marginRight: 12, color: '#8ba0c0' }}>{data.username || uname}</span>
        <span style={{ cursor: 'pointer', color: '#ff4d4f' }} onClick={logout}><LogoutOutlined /> 退出</span>
      </div>

      {/* Row 2: Tabs */}
      <div className="screen-tab-bar" style={{ display: 'flex' }}>
        {TABS.map(t => (
          <div key={t.key} className="screen-tab" onClick={() => setTab(t.key)} style={{
            padding: '10px 24px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6,
            background: tab === t.key ? 'rgba(1,218,243,0.10)' : 'transparent',
            color: tab === t.key ? '#01daf3' : 'rgba(217,229,228,0.5)',
            borderBottom: tab === t.key ? '2px solid #01daf3' : '2px solid transparent',
            fontWeight: tab === t.key ? 700 : 400,
          }}>{t.icon} {t.label}</div>
        ))}
      </div>

      {/* Body */}
      <ErrorBoundary>
      {tab === 'data' ? (
        <DataCenter key={`dc-${pid}-${bid}`} pid={pid} bid={bid} devices={data.devices || []} />
      ) : tab === 'maintain' ? (
        <MaintenanceCenter key={`mc-${pid}-${bid}`} pid={pid} bid={bid} devices={data.devices || []} />
      ) : tab === 'task' ? (
        <ScreenTaskCenter key={`tc-${bid}`} bid={bid} devices={data.devices || []} />
      ) : tab === 'decision' ? (
        <ScreenDecisionCenter key={`dec-${bid}`} bid={bid} devices={data.devices || []} />
      ) : tab === 'energy' ? (
        <ScreenEnergyCenter key={`ec-${bid}`} bid={bid} devices={data.devices || []} meterPower={data.meter_power || 0} />
      ) : tab === 'monitor' ? (
      <div style={{ display: 'flex', height: 'calc(100vh - 112px)', position: 'relative', zIndex: 10 }}>
        {/* LEFT */}
        <div style={{ width: 220, padding: 12, borderRight: '1px solid rgba(1,218,243,0.12)', overflowY: 'auto' }}>
          <div style={P}>
            <div style={PT}><EnvironmentOutlined /> 今日天气</div>
            {data.weather ? (
              <div style={{ textAlign: 'center', padding: '8px 0' }}>
                <div style={{ fontSize: 12, color: 'rgba(217,229,228,0.5)', letterSpacing: '0.02em' }}>{data.weather.city}</div>
                <div style={{ fontSize: 36, fontWeight: 700, color: '#01daf3', filter: 'drop-shadow(0 0 12px rgba(1,218,243,0.3))' }}>{data.weather.temp}°C</div>
                <div style={{ color: 'rgba(217,229,228,0.7)', fontSize: 13 }}>{data.weather.text} | 湿度 {data.weather.humidity}%</div>
                <div style={{ fontSize: 11, color: 'rgba(217,229,228,0.4)' }}>{data.weather.wind_dir} {data.weather.wind_scale}级</div>
              </div>
            ) : <div style={{ textAlign: 'center', padding: 16, color: 'rgba(217,229,228,0.4)' }}>暂无天气数据</div>}
          </div>

          <div style={P}>
            <div style={PT}><ClockCircleOutlined /> 定时任务</div>
            {(data.tasks || []).length === 0 ? <div style={E}>暂无任务</div> : (
              (data.tasks || []).slice(0, 6).map((t: any, i: number) => (
                <div key={i} style={{ fontSize: 11, padding: '3px 0', borderBottom: '1px solid rgba(1,218,243,0.06)' }}>
                  <span style={{ color: '#01daf3' }}>{t.time}</span> <span style={{ color: '#d9e5e4' }}>{t.device}</span>
                  <Tag color={t.enabled ? 'green' : 'default'} style={{ marginLeft: 4, fontSize: 10 }}>{t.enabled ? '启用' : '停用'}</Tag>
                </div>
              ))
            )}
          </div>

          <div style={P}>
            <div style={PT}><AlertOutlined /> 故障报警</div>
            {(data.alarms || []).length === 0 ? <div style={E}>无告警</div> : (
              (data.alarms || []).slice(0, 8).map((a: any, i: number) => (
                <div key={i} style={{ fontSize: 11, padding: '3px 0', borderBottom: '1px solid rgba(255,180,171,0.06)' }}>
                  <Tag color={a.level === 'critical' ? 'red' : 'orange'} style={{ fontSize: 10 }}>{a.level === 'critical' ? '严重' : '警告'}</Tag>
                  <span style={{ color: '#d9e5e4' }}>{a.device} {a.msg?.slice(0, 20)}</span>
                  <span style={{ float: 'right', color: 'rgba(217,229,228,0.3)', fontSize: 10 }}>{a.time}</span>
                </div>
              ))
            )}
          </div>
        </div>

        {/* CENTER - Topology */}
        <div style={{ flex: 1, padding: 16, overflowY: 'auto', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
          <div style={{ width: '100%', maxWidth: 800, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', marginBottom: 8 }}>
            <OneClickControl bid={bid} />
          </div>
          {!TOPO_ORDER.some(t => groups[t]?.length) ? (
            <div style={{ color: 'rgba(217,229,228,0.3)', padding: 40, textAlign: 'center' }}>暂无设备数据，请选择项目和楼宇</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 36, maxWidth: 800 }}>
              {/* Row 1: 冷却塔 — 塔间距2倍(16) */}
              <TopoRow items={groups['冷却塔']} color={TOPO_COLORS['冷却塔']} onOpen={openDevice} gap={16} />
              {/* Row 2: 冷却泵(竖) | 主机1(横排:阀门1+主机+阀门3) + 主机2(横排:阀门2+主机+阀门4) 竖排 | 冷冻泵(竖) */}
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 48 }}>
                {/* 冷却泵 — 泵间距3倍(36) */}
                <TopoCol items={groups['冷却泵']} color={TOPO_COLORS['冷却泵']} onOpen={openDevice} gap={36} />
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 36 }}>
                  {/* 主机1 + 阀门1(左) + 阀门3(右) — 横排 */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <div style={{ alignSelf: 'center', marginTop: 33 }}>
                      <TopoCol items={(groups['阀门'] || []).slice(0, 1)} color={TOPO_COLORS['阀门']} onOpen={openDevice} size="small" />
                    </div>
                    <TopoCol items={(groups['主机'] || []).slice(0, 1)} color={TOPO_COLORS['主机']} onOpen={openDevice} size="large" />
                    <div style={{ alignSelf: 'center', marginTop: 33 }}>
                      <TopoCol items={(groups['阀门'] || []).slice(2, 3)} color={TOPO_COLORS['阀门']} onOpen={openDevice} size="small" />
                    </div>
                  </div>
                  {/* 主机2 + 阀门2(左) + 阀门4(右) — 横排 */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <div style={{ alignSelf: 'center', marginTop: 33 }}>
                      <TopoCol items={(groups['阀门'] || []).slice(1, 2)} color={TOPO_COLORS['阀门']} onOpen={openDevice} size="small" />
                    </div>
                    <TopoCol items={(groups['主机'] || []).slice(1, 2)} color={TOPO_COLORS['主机']} onOpen={openDevice} size="large" />
                    <div style={{ alignSelf: 'center', marginTop: 33 }}>
                      <TopoCol items={(groups['阀门'] || []).slice(3, 4)} color={TOPO_COLORS['阀门']} onOpen={openDevice} size="small" />
                    </div>
                  </div>
                </div>
                {/* 冷冻泵 — 泵间距3倍(36) */}
                <TopoCol items={groups['冷冻泵']} color={TOPO_COLORS['冷冻泵']} onOpen={openDevice} gap={36} />
              </div>
              {/* Row 3: 二次泵 — 泵间距2倍(16) */}
              <div style={{ marginTop: 36 }}>
                <TopoRow items={groups['二次泵']} color={TOPO_COLORS['二次泵']} onOpen={openDevice} gap={16} />
              </div>
            </div>
          )}
        </div>

        {/* RIGHT */}
        <div style={{ width: 220, padding: 12, borderLeft: '1px solid rgba(1,218,243,0.12)', overflowY: 'auto' }}>
          <div style={P}>
            <div style={PT}><ThunderboltOutlined /> 能效概览</div>
            <div style={S}><span>节能率</span><span style={{ color: '#7fffd4', fontWeight: 700, filter: 'drop-shadow(0 0 4px rgba(127,255,212,0.4))' }}>{((data.saving_rate || 0) * 100).toFixed(1)}%</span></div>
            <div style={S}><span>节电量</span><span style={{ color: '#01daf3', fontWeight: 700, filter: 'drop-shadow(0 0 4px rgba(1,218,243,0.4))' }}>{(data.power_saved || 0).toFixed(1)} kWh</span></div>
            <div style={S}><span>节碳量</span><span style={{ color: '#84d4d3', fontWeight: 700, filter: 'drop-shadow(0 0 4px rgba(132,212,211,0.4))' }}>{(data.carbon_saved || 0).toFixed(1)} kg</span></div>
            <div style={S}><span>运行时长</span><span style={{ color: '#95d1d0', fontWeight: 700 }}>{data.running_days || 0} 天</span></div>
          </div>
          <div style={P}>
            <div style={PT}>电能统计</div>
            {(data.meters || []).length === 0 ? <div style={E}>暂无电表</div> : (
              (data.meters || []).map((m: any, i: number) => (
                <div key={i} style={S}><span style={{ fontSize: 11 }}>{m.name}</span><span style={{ color: '#01daf3', fontWeight: 600, fontSize: 12, filter: 'drop-shadow(0 0 3px rgba(1,218,243,0.3))' }}>{(Number(m.power) || 0).toFixed(1)} kW</span></div>
              ))
            )}
            <div style={{ ...S, borderTop: '1px solid rgba(1,218,243,0.12)', marginTop: 4, paddingTop: 4 }}>
              <span>总电能</span><span style={{ color: '#ff4d4f', fontWeight: 700 }}>{(Number(data.meter_power) || 0).toFixed(1)} kW</span>
            </div>
          </div>
        </div>
      </div>
      ) : (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 'calc(100vh - 112px)', color: '#5a7a9a', fontSize: 16 }}>
          {TABS.find(t => t.key === tab)?.label || '未知模块'} — 开发中
        </div>
      )}
      </ErrorBoundary>

      {/* Device Properties Modal */}
      <Modal
        title={<span style={{ color: '#01daf3', filter: 'drop-shadow(0 0 4px rgba(1,218,243,0.4))' }}>{devModal.dev?.name} — 设备属性</span>}
        open={devModal.open}
        onCancel={() => setDevModal({ open: false, dev: null })}
        footer={null}
        width={500}
        className="screen-modal"
        styles={{ body: { padding: 16 }, header: {} }}
      >
        {propsLoading ? <Spin /> : devProps.length === 0 ? (
          <div style={{ color: 'rgba(217,229,228,0.4)', textAlign: 'center', padding: 20 }}>该设备暂无属性配置</div>
        ) : (
          devProps.map((p: any) => (
            <div key={p.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '6px 0', borderBottom: '1px solid rgba(1,218,243,0.08)', fontSize: 13 }}>
              <span style={{ color: 'rgba(217,229,228,0.5)', minWidth: 80 }}>{p.prop_name}</span>
              {p.operation_type === '开关机' ? (
                <Switch size="small" checked={p.prop_value === '开机'} checkedChildren="开机" unCheckedChildren="关机"
                  onChange={v => doControl(devModal.dev?.id, v ? 'start' : 'stop')} />
              ) : p.operation_type === '模式选择' ? (
                <Select size="small" style={{ width: 140 }} value={p.prop_value || undefined}
                  onChange={v => doControl(devModal.dev?.id, 'mode_change', v)}
                  options={QUICK_MODES.map(o=>({value:o,label:o}))} />
              ) : p.operation_type === '数值' ? (
                <NumControl devId={devModal.dev?.id} prop={p} onSet={doControl} />
              ) : (
                <span style={{ color: '#c0d0e0', fontWeight: 600 }}>{p.prop_value || '-'} {p.unit || ''}</span>
              )}
            </div>
          ))
        )}
      </Modal>
    </div>
  );
}

const P: React.CSSProperties = { background: 'linear-gradient(135deg, rgba(255,255,255,0.06) 0%, rgba(255,255,255,0.02) 100%)', borderRadius: 10, padding: 12, marginBottom: 10, border: '1px solid rgba(1,218,243,0.2)', boxShadow: '0 8px 32px 0 rgba(0,0,0,0.2), inset 0 0 20px rgba(1,218,243,0.03)' };
const PT: React.CSSProperties = { fontSize: 12, fontWeight: 700, color: '#01daf3', marginBottom: 8, display: 'flex', alignItems: 'center', gap: 6, textTransform: 'uppercase', letterSpacing: '0.05em', filter: 'drop-shadow(0 0 5px rgba(1,218,243,0.4))' };
const S: React.CSSProperties = { display: 'flex', justifyContent: 'space-between', padding: '4px 0', fontSize: 12, color: '#d9e5e4' };
const E: React.CSSProperties = { textAlign: 'center', padding: 16, color: 'rgba(217,229,228,0.4)', fontSize: 12 };

// OneClickControl: 一键启停按钮（关联后台启停配置）
function OneClickControl({ bid }: { bid: number }) {
  const [plans, setPlans] = useState<any[]>([]);
  const [executing, setExecuting] = useState<number | null>(null);

  const fetchPlans = async () => {
    if (!bid) return;
    try {
      const r = await api.get('/startup-plans', { params: { building_id: bid } });
      setPlans(r.data || []);
    } catch {
      setPlans([]);
    }
  };

  useEffect(() => {
    if (bid) fetchPlans();
    else setPlans([]);
  }, [bid]);

  const doExecute = async (planId: number) => {
    setExecuting(planId);
    try {
      await api.post(`/startup-plans/${planId}/execute`);
      message.success('启停指令已发送');
    } catch {
      message.error('执行失败');
    } finally {
      setExecuting(null);
    }
  };

  if (!bid || plans.length === 0) return null;

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
      {plans.map((p: any) => (
        <Button
          key={p.id}
          size="small"
          type={p.plan_type === 'startup' ? 'primary' : 'default'}
          danger={p.plan_type === 'shutdown'}
          icon={p.plan_type === 'startup' ? <PlayCircleOutlined /> : <ThunderboltOutlined />}
          loading={executing === p.id}
          onClick={() => doExecute(p.id)}
          style={{ fontSize: 12, height: 28 }}
        >
          {p.name || (p.plan_type === 'startup' ? '一键开机' : '一键关机')}
        </Button>
      ))}
    </div>
  );
}

// NumControl: numeric value editor with set button
function NumControl({ devId, prop, onSet }: { devId: number; prop: any; onSet: (id: number, action: string, v?: string) => void }) {
  const [val, setVal] = useState(prop.prop_value ? parseFloat(prop.prop_value) : (prop.min_value ? parseFloat(prop.min_value) : 0));
  useEffect(() => {
    const newVal = prop.prop_value ? parseFloat(prop.prop_value) : (prop.min_value ? parseFloat(prop.min_value) : 0);
    setVal(newVal);
  }, [prop.prop_value, prop.min_value]);
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
      <InputNumber size="small" style={{ width: 100 }}
        min={prop.min_value ? parseFloat(prop.min_value) : undefined}
        max={prop.max_value ? parseFloat(prop.max_value) : undefined}
        step={0.1}
        value={val} onChange={v => setVal(v ?? 0)}
      />
      <span style={{ color: '#8ba0c0', fontSize: 11 }}>{prop.unit}</span>
      <Button size="small" type="primary" ghost onClick={() => onSet(devId, 'set_value', String(val))}>设置</Button>
    </div>
  );
}
