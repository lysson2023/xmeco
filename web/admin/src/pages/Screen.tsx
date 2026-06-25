import { useEffect, useState, useCallback } from 'react';
import { Select, Input, InputNumber, Button, Spin, Tag, message, Modal, Switch } from 'antd';
import {
  LogoutOutlined, UserOutlined, LockOutlined, ThunderboltOutlined,
  EnvironmentOutlined, ClockCircleOutlined, AlertOutlined,
  DashboardOutlined, DatabaseOutlined, ToolOutlined,
  ScheduleOutlined, RocketOutlined, FileTextOutlined, BulbOutlined,
  PlayCircleOutlined,
} from '@ant-design/icons';
import axios from 'axios';

// ---- Global keyframes for fault flashing ----
const styleSheet = document.createElement('style');
styleSheet.textContent = `@keyframes faultPulse { 0%,100% { box-shadow: 0 0 4px #ff4d4f; } 50% { box-shadow: 0 0 14px #ff4d4f; } }`;
document.head.appendChild(styleSheet);

// ---- Fully isolated API client (relative path, proxied in dev / same-origin in prod) ----
const api = axios.create({ baseURL: '/api/v1' });
const setAuth = (t: string) => { api.defaults.headers.common['Authorization'] = 'Bearer ' + t; };

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

  const TABS = [
    { key: 'monitor', icon: <DashboardOutlined />, label: '监控中心' },
    { key: 'data', icon: <DatabaseOutlined />, label: '数据中心' },
    { key: 'maintain', icon: <ToolOutlined />, label: '维保中心' },
    { key: 'task', icon: <ScheduleOutlined />, label: '任务中心' },
    { key: 'decision', icon: <RocketOutlined />, label: '决策中心' },
    { key: 'logs', icon: <FileTextOutlined />, label: '系统日志' },
    { key: 'energy', icon: <BulbOutlined />, label: '能耗中心' },
  ];

  const TOPO_ORDER = ['冷却塔', '冷却泵', '主机', '阀门', '冷冻泵', '二次泵'];
  const TOPO_COLORS: Record<string, string> = {
    '冷却塔': '#1677ff', '冷却泵': '#52c41a', '主机': '#fa8c16',
    '阀门': '#722ed1', '冷冻泵': '#13c2c2', '二次泵': '#eb2f96',
  };

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
      setLoggedIn(true);
    } catch { message.error('用户名或密码错误'); }
    finally { setLoginLoading(false); }
  };

  // ---- Init: check for saved token ----
  useEffect(() => {
    const saved = localStorage.getItem('screen_token');
    if (saved) {
      setAuth(saved);
      setUname(localStorage.getItem('screen_user') || '');
      setLoggedIn(true);
    }
  }, []);

  // ---- Fetch data ----
  const fetch = useCallback(async () => {
    if (!loggedIn) return;
    setLoading(true);
    try {
      const p: any = {}; if (pid) p.project_id = pid; if (bid) p.building_id = bid;
      const r = await api.get('/screen/data', { params: p });
      setData(r.data);
      if (!pid && r.data.projects?.[0]) setPid(r.data.projects[0].id);
    } catch { message.error('数据加载失败'); }
    finally { setLoading(false); }
  }, [loggedIn, pid, bid]);

  useEffect(() => { if (loggedIn) fetch(); }, [fetch]);
  useEffect(() => { if (loggedIn) { const t = setInterval(fetch, 30000); return () => clearInterval(t); } }, [fetch, loggedIn]);

  // ---- Logout ----
  const logout = () => {
    localStorage.removeItem('screen_token');
    localStorage.removeItem('screen_user');
    setLoggedIn(false);
  };

  // ---- Device click → load properties ----
  const openDevice = async (dev: any) => {
    setDevModal({ open: true, dev });
    setPropsLoading(true);
    try {
      const r = await api.get('/properties', { params: { device_id: dev.id } });
      setDevProps(r.data || []);
    } catch {
      setDevProps([]);
    } finally { setPropsLoading(false); }
  };

  // ---- Device control ----
  const doControl = async (devId: number, action: string, targetVal?: string) => {
    try {
      await api.post(`/devices/${devId}/control`, { action, target_value: targetVal || '' });
      message.success('指令已发送');
    } catch { message.error('控制失败'); }
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
      <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'linear-gradient(135deg, #0a1628, #1a2a4a)' }}>
        <div style={{ width: 380, padding: 40, background: 'rgba(255,255,255,0.05)', borderRadius: 16, border: '1px solid rgba(79,195,247,0.2)', textAlign: 'center', backdropFilter: 'blur(10px)' }}>
          <div style={{ fontSize: 32, marginBottom: 8 }}><ThunderboltOutlined style={{ color: '#4fc3f7' }} /></div>
          <h1 style={{ fontSize: 22, color: '#4fc3f7', fontWeight: 700, marginBottom: 4 }}>熊猫智控 XMECO</h1>
          <p style={{ fontSize: 13, color: '#8b949e', marginBottom: 28 }}>智慧能效大屏系统</p>
          <Input size="large" prefix={<UserOutlined style={{ color: '#8b949e' }} />} placeholder="用户名"
            value={uname} onChange={e => setUname(e.target.value)} onPressEnter={doLogin}
            style={{ marginBottom: 14, background: 'rgba(255,255,255,0.06)', border: '1px solid rgba(79,195,247,0.2)', color: '#c9d1d9' }} />
          <Input.Password size="large" prefix={<LockOutlined style={{ color: '#8b949e' }} />} placeholder="密码"
            value={pwd} onChange={e => setPwd(e.target.value)} onPressEnter={doLogin}
            style={{ marginBottom: 20, background: 'rgba(255,255,255,0.06)', border: '1px solid rgba(79,195,247,0.2)', color: '#c9d1d9' }} />
          <Button type="primary" size="large" block loading={loginLoading} onClick={doLogin}
            style={{ height: 44, background: '#4fc3f7', borderColor: '#4fc3f7', fontWeight: 600, fontSize: 15 }}>登 录</Button>
          <div style={{ marginTop: 20, fontSize: 11, color: '#484f58' }}>© 深圳市高海拔科技有限公司</div>
        </div>
      </div>
    );
  }

  // ==================== DASHBOARD ====================
  if (loading) {
    return <div style={{ textAlign: 'center', padding: '40vh 0', background: '#0a1628', minHeight: '100vh' }}><Spin size="large" /></div>;
  }

  return (
    <div style={{ minHeight: '100vh', background: '#0a1628', color: '#c0d0e0', fontFamily: 'system-ui' }}>
      {/* Row 1: Header */}
      <div style={{ display: 'flex', alignItems: 'center', padding: '6px 20px', background: '#0d1f3c', borderBottom: '1px solid #1a3455' }}>
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
      <div style={{ display: 'flex', background: '#0d1f3c', borderBottom: '1px solid #1a3455' }}>
        {TABS.map(t => (
          <div key={t.key} onClick={() => setTab(t.key)} style={{
            padding: '10px 24px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6,
            background: tab === t.key ? '#152d50' : 'transparent',
            color: tab === t.key ? '#00daf3' : '#8ba0c0',
            borderBottom: tab === t.key ? '2px solid #00daf3' : '2px solid transparent',
            fontWeight: tab === t.key ? 700 : 400,
          }}>{t.icon} {t.label}</div>
        ))}
      </div>

      {/* Body */}
      {tab !== 'monitor' ? (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: 'calc(100vh - 112px)', color: '#5a7a9a', fontSize: 16 }}>
          {TABS.find(t => t.key === tab)?.label} — 开发中
        </div>
      ) : (
      <div style={{ display: 'flex', height: 'calc(100vh - 112px)' }}>
        {/* LEFT */}
        <div style={{ width: 220, padding: 12, borderRight: '1px solid #1a3455', overflowY: 'auto' }}>
          <div style={P}>
            <div style={PT}><EnvironmentOutlined /> 今日天气</div>
            {data.weather ? (
              <div style={{ textAlign: 'center', padding: '8px 0' }}>
                <div style={{ fontSize: 12, color: '#8ba0c0' }}>{data.weather.city}</div>
                <div style={{ fontSize: 32, fontWeight: 700, color: '#00daf3' }}>{data.weather.temp}°C</div>
                <div>{data.weather.text} | 湿度 {data.weather.humidity}%</div>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>{data.weather.wind_dir} {data.weather.wind_scale}级</div>
              </div>
            ) : <div style={{ textAlign: 'center', padding: 16, color: '#8ba0c0' }}>暂无天气数据</div>}
          </div>

          <div style={P}>
            <div style={PT}><ClockCircleOutlined /> 定时任务</div>
            {(data.tasks || []).length === 0 ? <div style={E}>暂无任务</div> : (
              (data.tasks || []).slice(0, 6).map((t: any, i: number) => (
                <div key={i} style={{ fontSize: 11, padding: '3px 0', borderBottom: '1px solid #1a3455' }}>
                  <span style={{ color: '#00daf3' }}>{t.time}</span> {t.device}
                  <Tag color={t.enabled ? 'green' : 'default'} style={{ marginLeft: 4, fontSize: 10 }}>{t.enabled ? '启用' : '停用'}</Tag>
                </div>
              ))
            )}
          </div>

          <div style={P}>
            <div style={PT}><AlertOutlined /> 故障报警</div>
            {(data.alarms || []).length === 0 ? <div style={E}>无告警</div> : (
              (data.alarms || []).slice(0, 8).map((a: any, i: number) => (
                <div key={i} style={{ fontSize: 11, padding: '3px 0', borderBottom: '1px solid #1a3455' }}>
                  <Tag color={a.level === 'critical' ? 'red' : 'orange'} style={{ fontSize: 10 }}>{a.level === 'critical' ? '严重' : '警告'}</Tag>
                  <span style={{ color: '#c0d0e0' }}>{a.device} {a.msg?.slice(0, 20)}</span>
                  <span style={{ float: 'right', color: '#5a7a9a', fontSize: 10 }}>{a.time}</span>
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
            <div style={{ color: '#5a7a9a', padding: 40 }}>暂无设备数据，请选择项目和楼宇</div>
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
        <div style={{ width: 220, padding: 12, borderLeft: '1px solid #1a3455', overflowY: 'auto' }}>
          <div style={P}>
            <div style={PT}><ThunderboltOutlined /> 能效概览</div>
            <div style={S}><span>节能率</span><span style={{ color: '#52c41a', fontWeight: 700 }}>{((data.saving_rate || 0) * 100).toFixed(1)}%</span></div>
            <div style={S}><span>节电量</span><span style={{ color: '#00daf3', fontWeight: 700 }}>{(data.power_saved || 0).toFixed(1)} kWh</span></div>
            <div style={S}><span>节碳量</span><span style={{ color: '#13c2c2', fontWeight: 700 }}>{(data.carbon_saved || 0).toFixed(1)} kg</span></div>
            <div style={S}><span>运行时长</span><span style={{ color: '#fa8c16', fontWeight: 700 }}>{data.running_days || 0} 天</span></div>
          </div>
          <div style={P}>
            <div style={PT}>电能统计</div>
            {(data.meters || []).length === 0 ? <div style={E}>暂无电表</div> : (
              (data.meters || []).map((m: any, i: number) => (
                <div key={i} style={S}><span style={{ fontSize: 11 }}>{m.name}</span><span style={{ color: '#00daf3', fontWeight: 600, fontSize: 12 }}>{m.power.toFixed(1)} kW</span></div>
              ))
            )}
            <div style={{ ...S, borderTop: '1px solid #1a3455', marginTop: 4, paddingTop: 4 }}>
              <span>总功率</span><span style={{ color: '#ff4d4f', fontWeight: 700 }}>{(data.meter_power || 0).toFixed(1)} kW</span>
            </div>
          </div>
        </div>
      </div>
      )}

      {/* Device Properties Modal */}
      <Modal
        title={<span style={{ color: '#00daf3' }}>{devModal.dev?.name} — 设备属性</span>}
        open={devModal.open}
        onCancel={() => setDevModal({ open: false, dev: null })}
        footer={null}
        width={500}
        styles={{ body: { background: '#0d1f3c', padding: 16 }, header: { background: '#0d1f3c', borderBottom: '1px solid #1a3455' } }}
      >
        {propsLoading ? <Spin /> : devProps.length === 0 ? (
          <div style={{ color: '#8ba0c0', textAlign: 'center', padding: 20 }}>该设备暂无属性配置</div>
        ) : (
          devProps.map((p: any) => (
            <div key={p.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '6px 0', borderBottom: '1px solid #1a3455', fontSize: 13 }}>
              <span style={{ color: '#8ba0c0', minWidth: 80 }}>{p.prop_name}</span>
              {p.operation_type === '开关机' ? (
                <Switch size="small" checked={p.prop_value === '开机'} checkedChildren="开机" unCheckedChildren="关机"
                  onChange={v => doControl(devModal.dev?.id, v ? 'start' : 'stop')} />
              ) : p.operation_type === '模式选择' ? (
                <Select size="small" style={{ width: 140 }} value={p.prop_value || undefined}
                  onChange={v => doControl(devModal.dev?.id, 'mode_change', v)}
                  options={['制冷','制热','除湿','送风'].map(o=>({value:o,label:o}))} />
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

const P: React.CSSProperties = { background: '#0d1f3c', borderRadius: 6, padding: 10, marginBottom: 10, border: '1px solid #1a3455' };
const PT: React.CSSProperties = { fontSize: 13, fontWeight: 700, color: '#00daf3', marginBottom: 8, display: 'flex', alignItems: 'center', gap: 6 };
const S: React.CSSProperties = { display: 'flex', justifyContent: 'space-between', padding: '4px 0', fontSize: 12 };
const E: React.CSSProperties = { textAlign: 'center', padding: 12, color: '#5a7a9a', fontSize: 12 };

// TopoRow: horizontal row of devices
function TopoRow({ items, color, onOpen, size, gap: rowGap }: { items: any[]; color: string; onOpen: (d: any) => void; size?: 'normal' | 'small' | 'large'; gap?: number }) {
  if (!items || items.length === 0) return null;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6 }}>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: rowGap ?? 4, justifyContent: 'center' }}>
        {items.map((d: any) => <TopoDevice key={d.id} d={d} color={color} onOpen={onOpen} size={size} />)}
      </div>
    </div>
  );
}

// TopoCol: vertical column of devices
function TopoCol({ items, color, onOpen, size, gap: colGap }: { items: any[]; color: string; onOpen: (d: any) => void; size?: 'normal' | 'small' | 'large'; gap?: number }) {
  if (!items || items.length === 0) return null;
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6 }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: colGap ?? 4, alignItems: 'center' }}>
        {items.map((d: any) => <TopoDevice key={d.id} d={d} color={color} onOpen={onOpen} size={size} />)}
      </div>
    </div>
  );
}

// TopoDevice: single device block
// States: 故障(红色闪烁) | 在线+开机(亮色实心方■) | 在线+关机(暗色空心方□) | 离线+开机(灰色虚线实心圆●) | 离线+关机(灰色虚线空心圆○)
function TopoDevice({ d, color, onOpen, size }: { d: any; color: string; onOpen: (d: any) => void; size?: 'normal' | 'small' | 'large' }) {
  const isOnline = d.status === '在线';
  const devStatus = d.device_status || '';
  const isFault = devStatus === '故障';
  const isOn = devStatus !== '关机' && devStatus !== '停机' && devStatus !== '';
  const isSmall = size === 'small';
  const isLarge = size === 'large';

  let bg: string;
  let border: string;
  let opacity: number;
  let label: string;
  let labelColor: string;
  let anim: string;

  if (isFault) {
    bg = '#5c1a1a';
    border = '2px solid #ff4d4f';
    opacity = 1;
    label = '✕';
    labelColor = '#ff4d4f';
    anim = 'faultPulse 1.2s ease-in-out infinite';
  } else if (isOnline && isOn) {
    bg = color || '#666';
    border = '2px solid rgba(255,255,255,0.3)';
    opacity = 1;
    label = '■';
    labelColor = '#fff';
    anim = '';
  } else if (isOnline && !isOn) {
    bg = '#3a4a5a';
    border = '2px solid #5a7a9a';
    opacity = 0.8;
    label = '□';
    labelColor = '#7a9aba';
    anim = '';
  } else if (!isOnline && isOn) {
    bg = '#5a5a5a';
    border = '2px dashed #666';
    opacity = 0.6;
    label = '●';
    labelColor = '#888';
    anim = '';
  } else {
    bg = '#2a2a2a';
    border = '2px dashed #555';
    opacity = 0.5;
    label = '○';
    labelColor = '#555';
    anim = '';
  }

  const boxW = isLarge ? 160 : isSmall ? 44 : 56;
  const boxH = isLarge ? 110 : isSmall ? 44 : 56;
  const fontSize = isLarge ? 14 : isSmall ? 9 : 10;
  const labelFontSize = isLarge ? 18 : isSmall ? 10 : 12;

  return (
    <div title={d.key_info ? d.name + ': ' + d.key_info : '点击查看属性'} onClick={() => onOpen(d)} style={{
      width: boxW, height: boxH,
      borderRadius: 8, cursor: 'pointer',
      background: bg,
      display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
      color: '#fff', fontSize, fontWeight: 600,
      border, opacity,
      animation: anim,
      transition: 'transform 0.15s',
    }} onMouseEnter={e => (e.currentTarget.style.transform = 'scale(1.05)')} onMouseLeave={e => (e.currentTarget.style.transform = 'scale(1)')}>
      {!isLarge && <div style={{ fontSize: labelFontSize, color: labelColor }}>{label}</div>}
      <div style={{ lineHeight: 1.2, textAlign: 'center', fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: boxW - 8 }}>{d.name.length > 4 ? d.name.slice(0, 4) + '…' : d.name}</div>
      {isLarge && d.key_info && (() => {
        const lines = d.key_info.split(/\s*\|\s*/).filter(Boolean);
        return (
          <div style={{ fontSize: 11, color: 'rgba(255,255,255,0.85)', marginTop: 4, lineHeight: 1.5, textAlign: 'center', maxWidth: 150 }}>
            {lines.map((line: string, i: number) => <div key={i}>{line}</div>)}
          </div>
        );
      })()}
    </div>
  );
}

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
