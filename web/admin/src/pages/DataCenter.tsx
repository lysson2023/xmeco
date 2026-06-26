import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { Tabs, Select, DatePicker, Card, Spin, Empty, Tag, message, ConfigProvider } from 'antd';
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend,
  ResponsiveContainer, ReferenceArea,
} from 'recharts';
import {
  ThunderboltOutlined, HistoryOutlined, ClockCircleOutlined,
  CheckCircleOutlined, MinusCircleOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import locale from 'antd/locale/zh_CN';
import { api } from '../api/screenClient';

dayjs.locale('zh-cn');

import { DATA_ORDER, DATA_COLORS, CHART_COLORS } from '../utils/constants';

// ---- Shared dark-theme styles ----
const darkCard: React.CSSProperties = {
  background: '#0d1f3c', borderRadius: 6, padding: 12, border: '1px solid #1a3455',
};

interface DataCenterProps {
  pid: number;
  bid: number;
  devices: any[];
}

export default function DataCenter({ pid, bid, devices }: DataCenterProps) {
  const [subTab, setSubTab] = useState('realtime');

  return (
    <ConfigProvider locale={locale} theme={{ token: { colorPrimary: '#00daf3' } }}>
    <div style={{ height: 'calc(100vh - 112px)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      {/* Sub-tabs */}
      <div style={{ display: 'flex', background: '#0d1f3c', borderBottom: '1px solid #1a3455', paddingLeft: 16 }}>
        {[
          { key: 'realtime', icon: <ThunderboltOutlined />, label: '实时数据' },
          { key: 'history', icon: <HistoryOutlined />, label: '历史数据' },
        ].map(t => (
          <div key={t.key} onClick={() => setSubTab(t.key)} style={{
            padding: '10px 24px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6,
            background: subTab === t.key ? '#152d50' : 'transparent',
            color: subTab === t.key ? '#00daf3' : '#8ba0c0',
            borderBottom: subTab === t.key ? '2px solid #00daf3' : '2px solid transparent',
            fontWeight: subTab === t.key ? 700 : 400,
          }}>{t.icon} {t.label}</div>
        ))}
      </div>

      {/* Content */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        {subTab === 'realtime'
          ? <RealtimePanel pid={pid} bid={bid} devices={devices} />
          : <HistoryPanel pid={pid} bid={bid} devices={devices} />
        }
      </div>
    </div>
    </ConfigProvider>
  );
}

// ======================== REALTIME PANEL ========================
function RealtimePanel({ pid, bid, devices }: { pid: number; bid: number; devices: any[] }) {
  const [allProps, setAllProps] = useState<Record<number, any[]>>({});
  const [activeTab, setActiveTab] = useState('');
  const fetchedRef = useRef(false);
  const didAutoSelect = useRef(false);
  const mountedRef = useRef(true);

  // Group devices by type — 用 useMemo 缓存，只有 devices 内容真正变化时才重新计算
  // 避免每次轮询导致的引用变化触发下游 useEffect
  const grouped = useMemo(() => groupDevices(devices), [devices]);
  const groupKeys = useMemo(() => Object.keys(grouped), [grouped]);

  // Fetch all properties — 只依赖 pid/bid
  const fetchAllProps = useCallback(async (devs: any[]) => {
    if (!devs.length || fetchedRef.current) return;
    fetchedRef.current = true;
    try {
      const results = await Promise.allSettled(
        devs.map((d) => api.get('/properties', { params: { device_id: d.id } })),
      );
      if (!mountedRef.current) return;
      const map: Record<number, any[]> = {};
      devs.forEach((d, i) => {
        const r = results[i];
        map[d.id] = (r.status === 'fulfilled' && r.value?.data) ? r.value.data : [];
      });
      setAllProps(map);
    } catch { /* ignore */ }
  }, []);

  // Fetch properties when pid/bid changes OR when devices first become non-empty
  // 依赖 devices.length 确保异步加载的设备列表到达后能正确获取属性
  const devCount = devices.length;
  const devIds = useMemo(() => devices.map(d => d.id).join(','), [devices]);
  useEffect(() => {
    mountedRef.current = true;
    fetchedRef.current = false;
    if (devCount > 0) {
      fetchAllProps(devices);
    }
    return () => { mountedRef.current = false; };
  }, [pid, bid, devCount, devIds]);

  // Auto-select first group exactly once
  useEffect(() => {
    if (didAutoSelect.current) return;
    if (groupKeys.length > 0) {
      didAutoSelect.current = true;
      setActiveTab(groupKeys[0]);
    }
  }, [groupKeys]);

  // Build tab items for horizontal layout — 不再用 Ant Design Tabs，改用手动 div
  // 避免 30s 轮询导致 items 引用变化时 Tabs 内部重置 activeKey

  if (!bid) {
    return <div style={{ textAlign: 'center', padding: 60, color: '#5a7a9a', fontSize: 15 }}>请先选择楼宇</div>;
  }

  if (groupKeys.length === 0) {
    return <Empty description={<span style={{ color: '#5a7a9a' }}>该楼宇暂无设备数据</span>} style={{ marginTop: 60 }} />;
  }

  // 确保 activeTab 仍然有效（防止设备列表变化后 activeTab 失效）
  const validTab = groupKeys.includes(activeTab) ? activeTab : groupKeys[0];
  const activeDevs = grouped[validTab] || [];
  const activeColor = DATA_COLORS[validTab] || '#8ba0c0';

  return (
    <div style={{ padding: '8px 16px' }}>
      {/* 手动 Tab 栏 */}
      <div style={{ display: 'flex', borderBottom: '1px solid #1a3455', marginBottom: 8, flexWrap: 'wrap' }}>
        {groupKeys.map((gkey) => {
          const devs = grouped[gkey];
          const color = DATA_COLORS[gkey] || '#8ba0c0';
          const isActive = validTab === gkey;
          return (
            <div
              key={gkey}
              onClick={() => setActiveTab(gkey)}
              style={{
                padding: '10px 20px',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: 6,
                color: isActive ? '#fff' : '#8ba0c0',
                fontWeight: isActive ? 700 : 400,
                fontSize: 13,
                borderBottom: isActive ? '2px solid #00daf3' : '2px solid transparent',
                background: isActive ? 'rgba(0,218,243,0.08)' : 'transparent',
                transition: 'all 0.2s',
              }}
            >
              <span style={{ display: 'inline-block', width: 8, height: 8, borderRadius: 2, background: color, verticalAlign: 'middle' }} />
              {gkey}（{devs.length}）
            </div>
          );
        })}
      </div>

      {/* 设备卡片 */}
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, padding: '8px 0' }}>
        {activeDevs.map((d: any) => (
          <DeviceCard key={d.id} device={d} properties={allProps[d.id] || []} color={activeColor} />
        ))}
      </div>
    </div>
  );
}

// ---- Device Card ----
function DeviceCard({ device, properties, color }: { device: any; properties: any[]; color: string }) {
  const isOnline = device.status === '在线';
  const devStatus = device.device_status || '';
  const isFault = devStatus === '故障';
  const isOn = devStatus !== '关机' && devStatus !== '停机' && devStatus !== '';
  const isSensor = device.type === '温湿度传感器';

  // 温湿度传感器：从 sensor-data API 加载固定属性
  const [sensorInfo, setSensorInfo] = useState<any>(null);
  useEffect(() => {
    if (!isSensor) return;
    let mounted = true;
    api.get('/devices/' + device.id + '/sensor-data').then(r => {
      if (mounted) setSensorInfo(r.data);
    }).catch(() => {});
    return () => { mounted = false; };
  }, [device.id]);

  const statusColor = isFault ? '#ff4d4f' : isOnline && isOn ? '#52c41a' : isOnline ? '#faad14' : '#888';
  const statusText = isFault ? '故障' : isOnline ? (isOn ? '运行中' : '已停机') : '离线';

  // 温湿度传感器固定显示的五个属性
  const sensorRows = sensorInfo ? [
    { label: '温度(℃)', value: sensorInfo.temperature ? sensorInfo.temperature.toFixed(1) : '-' },
    { label: '湿度(%)', value: sensorInfo.humidity ? sensorInfo.humidity.toFixed(1) : '-' },
    { label: '电压(V)', value: sensorInfo.voltage ? sensorInfo.voltage.toFixed(1) : '-' },
    { label: '信号强度(dbm)', value: sensorInfo.signal_strength ? sensorInfo.signal_strength.toFixed(0) : '-' },
    { label: '时间间隔(M)', value: sensorInfo.interval_minutes || '-' },
  ] : [];

  return (
    <div style={{
      ...darkCard, minWidth: 240, maxWidth: 320, flex: '1 1 240px',
      borderTop: `3px solid ${color}`,
    }}>
      {/* Device header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <span style={{ color: '#c0d0e0', fontWeight: 700, fontSize: 14 }}>{device.name}</span>
        <Tag color={isFault ? 'red' : isOnline ? (isOn ? 'green' : 'gold') : 'default'} style={{ fontSize: 10, margin: 0 }}>
          {statusText}
        </Tag>
      </div>

      {/* Properties */}
      {isSensor ? (
        // 温湿度传感器：固定显示温度/湿度/电压/信号强度/时间间隔
        sensorRows.length === 0 ? (
          <div style={{ color: '#5a7a9a', fontSize: 12, textAlign: 'center', padding: 8 }}>加载中...</div>
        ) : (
          sensorRows.map((row, i) => (
            <div key={i} style={{ display: 'flex', justifyContent: 'space-between', padding: '3px 0', borderBottom: '1px solid rgba(26,52,85,0.5)', fontSize: 12 }}>
              <span style={{ color: '#8ba0c0', flex: '0 0 auto', maxWidth: '55%', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{row.label}</span>
              <span style={{ color: '#00daf3', fontWeight: 600, textAlign: 'right', flex: 1 }}>{row.value}</span>
            </div>
          ))
        )
      ) : properties.length === 0 ? (
        <div style={{ color: '#5a7a9a', fontSize: 12, textAlign: 'center', padding: 8 }}>暂无属性配置</div>
      ) : (
        properties.map((p: any) => (
          <div key={p.id} style={{ display: 'flex', justifyContent: 'space-between', padding: '3px 0', borderBottom: '1px solid rgba(26,52,85,0.5)', fontSize: 12 }}>
            <span style={{ color: '#8ba0c0', flex: '0 0 auto', maxWidth: '55%', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.prop_name}</span>
            <span style={{ color: '#00daf3', fontWeight: 600, textAlign: 'right', flex: 1 }}>
              {p.prop_value || '-'}{p.unit ? ` ${p.unit}` : ''}
            </span>
          </div>
        ))
      )}
    </div>
  );
}

// ======================== HISTORY PANEL ========================
function HistoryPanel({ pid, bid, devices }: { pid: number; bid: number; devices: any[] }) {
  const [selDevice, setSelDevice] = useState<number | null>(null);
  const [dateRange, setDateRange] = useState<[any, any]>([dayjs().startOf('day'), dayjs()]);
  const [loading, setLoading] = useState(false);
  const [chartData, setChartData] = useState<any[]>([]);
  const [metrics, setMetrics] = useState<string[]>([]);
  const [onOffSegments, setOnOffSegments] = useState<any[]>([]);
  const [selectedMetrics, setSelectedMetrics] = useState<string[]>([]);
  const [deviceName, setDeviceName] = useState('');
  const mountedRef = useRef(true);
  const hasUserSelected = useRef(false); // 标记用户是否手动操作过指标选择
  const fetchSeqRef = useRef(0); // 请求序号，防止旧请求覆盖新数据

  const buildingDevices = devices;

  const doQuery = useCallback(async () => {
    if (!selDevice) return;
    setLoading(true);
    const seq = ++fetchSeqRef.current;
    try {
      const start = dateRange[0] ? dateRange[0].format('YYYY-MM-DD') : dayjs().startOf('day').format('YYYY-MM-DD');
      const end = dateRange[1] ? dateRange[1].format('YYYY-MM-DD 23:59:59') : dayjs().format('YYYY-MM-DD 23:59:59');

      const teleRes = await api.get('/logs/telemetry', {
        params: { device_id: selDevice, interval: 'minute', start, end },
      });
      if (seq !== fetchSeqRef.current || !mountedRef.current) return;
      const raw: any[] = teleRes.data || [];

      const metricSet = new Set<string>();
      raw.forEach((r: any) => metricSet.add(r.metric));
      const metricList = Array.from(metricSet).sort();
      setMetrics(metricList);

      // 仅在用户未手动操作过指标选择时，自动选择前 4 个
      if (!hasUserSelected.current && selectedMetrics.length === 0) {
        setSelectedMetrics(metricList.slice(0, Math.min(4, metricList.length)));
      }

      const pivotMap: Record<string, any> = {};
      raw.forEach((row: any) => {
        const ts = dayjs(row.ts).format('MM-DD HH:mm');
        if (!pivotMap[ts]) pivotMap[ts] = { ts };
        pivotMap[ts][row.metric] = row.avg;
      });
      const pivoted = Object.values(pivotMap).sort((a: any, b: any) =>
        a.ts.localeCompare(b.ts),
      );
      if (seq !== fetchSeqRef.current || !mountedRef.current) return;
      setChartData(pivoted);

      try {
        const ctrlRes = await api.get('/logs/controls', {
          params: { device_id: selDevice, start, end },
        });
        if (seq !== fetchSeqRef.current || !mountedRef.current) return;
        const ctrls: any[] = ctrlRes.data || [];
        const segments = buildOnOffSegments(ctrls);
        setOnOffSegments(segments);
      } catch {
        if (mountedRef.current) setOnOffSegments([]);
      }
    } catch (err: any) {
      if (seq !== fetchSeqRef.current || !mountedRef.current) return;
      if (err?.response?.status !== 401) message.error('数据加载失败');
      setChartData([]);
      setOnOffSegments([]);
    } finally {
      if (seq === fetchSeqRef.current && mountedRef.current) setLoading(false);
    }
  }, [selDevice, dateRange, selectedMetrics]);

  useEffect(() => {
    mountedRef.current = true;
    if (selDevice) {
      const dev = devices.find((d: any) => d.id === selDevice);
      setDeviceName(dev?.name || '');
      setChartData([]);
      setMetrics([]);
      setOnOffSegments([]);
      hasUserSelected.current = false; // 切换设备时重置用户选择标记
      doQuery();
    }
    return () => { mountedRef.current = false; };
  }, [selDevice, dateRange]);

  const toggleMetric = (m: string) => {
    hasUserSelected.current = true; // 标记用户已手动操作
    setSelectedMetrics((prev) =>
      prev.includes(m) ? prev.filter((x) => x !== m) : [...prev, m],
    );
  };

  if (!bid) {
    return <div style={{ textAlign: 'center', padding: 60, color: '#5a7a9a', fontSize: 15 }}>请先选择楼宇</div>;
  }

  return (
    <div style={{ padding: 16 }}>
      {/* Controls */}
      <div style={{ display: 'flex', gap: 12, marginBottom: 12, alignItems: 'center', flexWrap: 'wrap' }}>
        <div>
          <div style={{ color: '#8ba0c0', fontSize: 11, marginBottom: 2 }}>设备</div>
          <Select
            style={{ width: 180 }}
            placeholder="选择设备"
            value={selDevice}
            onChange={(v) => setSelDevice(v)}
            options={buildingDevices.map((d: any) => ({ value: d.id, label: d.name }))}
          />
        </div>
        <div>
          <div style={{ color: '#8ba0c0', fontSize: 11, marginBottom: 2 }}>时间范围</div>
          <DatePicker.RangePicker
            value={dateRange as any}
            onChange={(v: any) => v && setDateRange(v)}
            showTime={{ format: 'HH:mm' }}
            format="YYYY-MM-DD HH:mm"
            locale={locale.DatePicker}
            style={{ background: '#0d1f3c', border: '1px solid #1a3455' }}
          />
        </div>
        {selDevice && (
          <span style={{ color: '#8ba0c0', fontSize: 13, paddingTop: 18 }}>
            <ClockCircleOutlined /> {deviceName}
          </span>
        )}
      </div>

      {/* Metric selectors */}
      {metrics.length > 0 && (
        <div style={{ marginBottom: 12, display: 'flex', flexWrap: 'wrap', gap: 6, alignItems: 'center' }}>
          <span style={{ color: '#8ba0c0', fontSize: 12, marginRight: 4 }}>指标:</span>
          {metrics.map((m) => (
            <Tag
              key={m}
              color={selectedMetrics.includes(m) ? 'blue' : 'default'}
              style={{ cursor: 'pointer', fontSize: 11 }}
              onClick={() => toggleMetric(m)}
            >
              {m}
            </Tag>
          ))}
        </div>
      )}

      {/* Chart */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: 60 }}><Spin size="large" /></div>
      ) : !selDevice ? (
        <div style={{ textAlign: 'center', padding: 60, color: '#5a7a9a', fontSize: 15 }}>请选择设备查看历史数据</div>
      ) : chartData.length === 0 ? (
        <Empty description={<span style={{ color: '#5a7a9a' }}>该时间段暂无数据</span>} style={{ marginTop: 40 }} />
      ) : (
        <div style={{ ...darkCard, padding: 16 }}>
          {/* On/Off status summary */}
          {onOffSegments.length > 0 && (
            <div style={{ marginBottom: 12, display: 'flex', gap: 8, alignItems: 'center', fontSize: 12 }}>
              <span style={{ color: '#8ba0c0' }}>运行时段:</span>
              {onOffSegments.slice(0, 8).map((seg: any, i: number) => (
                <Tag key={i} color={seg.type === 'on' ? 'green' : 'red'} style={{ fontSize: 10 }}>
                  {seg.type === 'on' ? <CheckCircleOutlined /> : <MinusCircleOutlined />}
                  {' '}{seg.start} ~ {seg.end}
                </Tag>
              ))}
              {onOffSegments.length > 8 && <span style={{ color: '#5a7a9a' }}>+{onOffSegments.length - 8}段</span>}
            </div>
          )}

          <ResponsiveContainer width="100%" height={380}>
            <LineChart data={chartData} margin={{ top: 5, right: 20, left: 10, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#1a3455" />
              <XAxis dataKey="ts" stroke="#5a7a9a" tick={{ fontSize: 10 }} interval="preserveStartEnd" />
              <YAxis stroke="#5a7a9a" tick={{ fontSize: 10 }} />
              <Tooltip
                contentStyle={{ background: '#0d1f3c', border: '1px solid #1a3455', borderRadius: 6, color: '#c0d0e0', fontSize: 12 }}
                labelStyle={{ color: '#00daf3', fontWeight: 600 }}
              />
              <Legend wrapperStyle={{ fontSize: 12 }} />

              {/* On/Off reference areas */}
              {onOffSegments.filter((s: any) => s.type === 'off').map((seg: any, i: number) => (
                <ReferenceArea
                  key={`off-${i}`}
                  x1={seg.start} x2={seg.end}
                  fill="#ff4d4f" fillOpacity={0.06}
                  label={{ value: '关机', position: 'insideTop', fill: '#ff4d4f', fontSize: 10 }}
                />
              ))}

              {/* Metric lines */}
              {selectedMetrics.map((m, i) => (
                <Line
                  key={m}
                  type="monotone"
                  dataKey={m}
                  stroke={CHART_COLORS[i % CHART_COLORS.length]}
                  strokeWidth={1.5}
                  dot={false}
                  activeDot={{ r: 3 }}
                  connectNulls
                />
              ))}
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

// ======================== HELPERS ========================

/** Group devices by device_type using DATA_ORDER. */
function groupDevices(devices: any[]): Record<string, any[]> {
  const groups: Record<string, any[]> = {};
  const others: any[] = [];
  devices.forEach((d: any) => {
    const key = d.type;
    if (DATA_ORDER.includes(key)) {
      if (!groups[key]) groups[key] = [];
      groups[key].push(d);
    } else {
      others.push(d);
    }
  });
  // Sort groups by DATA_ORDER
  const ordered: Record<string, any[]> = {};
  DATA_ORDER.forEach((t) => { if (groups[t]) ordered[t] = groups[t]; });
  if (others.length) ordered['其他'] = others;
  return ordered;
}

/** Build on/off time segments from control records */
function buildOnOffSegments(ctrls: any[]): any[] {
  const actions = ctrls
    .filter((c: any) => c.control_value === '开机' || c.control_value === '关机')
    .sort((a: any, b: any) => dayjs(a.created_at).valueOf() - dayjs(b.created_at).valueOf());

  const segments: any[] = [];
  let onStart: string | null = null;

  for (const act of actions) {
    const ts = dayjs(act.created_at).format('MM-DD HH:mm');
    if (act.control_value === '开机') {
      if (onStart) {
        segments.push({ type: 'on', start: onStart, end: ts });
      }
      onStart = ts;
    } else {
      if (onStart) {
        segments.push({ type: 'on', start: onStart, end: ts });
        onStart = null;
      }
      segments.push({ type: 'off', start: ts, end: ts });
    }
  }
  if (onStart) {
    segments.push({ type: 'on', start: onStart, end: '至今' });
  }

  return segments;
}
