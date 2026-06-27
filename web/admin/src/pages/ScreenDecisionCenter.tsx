import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { Spin, DatePicker, Radio, Tag } from 'antd';
import { ThunderboltOutlined, AlertOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import {
  LineChart, Line, BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip, Legend, ResponsiveContainer,
} from 'recharts';
import { api } from '../api/screenClient';

// ---- dark theme chart colors ----
const CYAN = '#00daf3';
const ORANGE = '#fa8c16';
const RED = '#ff4d4f';
const AXIS_COLOR = '#5a7a9a';
const GRID_COLOR = '#1a3455';

// ---- Power-related metric keywords ----
const POWER_KEYWORDS = ['power', 'energy', 'kwh', '功率', '电量', '电度', '有功', '电能'];

const darkCard: React.CSSProperties = {
  background: '#0d1f3c', borderRadius: 6, padding: 16, border: '1px solid #1a3455',
};

const sectionTitle: React.CSSProperties = {
  fontSize: 14, fontWeight: 700, color: '#00daf3', marginBottom: 12,
  display: 'flex', alignItems: 'center', gap: 6,
};

// ---- period options ----
const PERIODS = [
  { label: '日', value: 'day' },
  { label: '周', value: 'week' },
  { label: '月', value: 'month' },
  { label: '季', value: 'quarter' },
  { label: '半年', value: 'halfyear' },
  { label: '一年', value: 'year' },
];

function periodToRange(period: string): { start: dayjs.Dayjs; interval: string } {
  const now = dayjs();
  switch (period) {
    case 'day': return { start: now.startOf('day'), interval: 'hour' };
    case 'week': return { start: now.subtract(6, 'day').startOf('day'), interval: 'day' };
    case 'month': return { start: now.subtract(29, 'day').startOf('day'), interval: 'day' };
    case 'quarter': return { start: now.subtract(89, 'day').startOf('day'), interval: 'day' };
    case 'halfyear': return { start: now.subtract(179, 'day').startOf('day'), interval: 'week' };
    case 'year': return { start: now.subtract(364, 'day').startOf('day'), interval: 'month' };
    default: return { start: now.subtract(6, 'day').startOf('day'), interval: 'day' };
  }
}

function periodToAlarmInterval(period: string): string {
  switch (period) {
    case 'day': return 'hour';
    case 'week': return 'day';
    case 'month': return 'day';
    case 'quarter': return 'week';
    case 'halfyear': return 'week';
    case 'year': return 'month';
    default: return 'day';
  }
}

interface ScreenDecisionCenterProps {
  bid: number;
  devices: any[];
}

const SUB_TABS = [
  { key: 'power', icon: <ThunderboltOutlined />, label: '电量统计' },
  { key: 'fault', icon: <AlertOutlined />, label: '故障统计' },
];

export default function ScreenDecisionCenter({ bid, devices }: ScreenDecisionCenterProps) {
  const [subTab, setSubTab] = useState<'power' | 'fault'>('power');

  // ---- device type map (memoized to prevent fetchFaults useCallback infinite loop) ----
  const deviceTypeMap = useMemo(() => {
    const m: Record<number, string> = {};
    devices.forEach((d: any) => { m[d.id] = d.type || ''; });
    return m;
  }, [devices]);

  // ==================== POWER STATS ====================
  const [powerMetrics, setPowerMetrics] = useState<string[]>([]);
  const [selectedMetric, setSelectedMetric] = useState<string>('');
  const selectedMetricRef = useRef(selectedMetric);
  selectedMetricRef.current = selectedMetric;

  // -- intraday comparison --
  const [baseDate, setBaseDate] = useState<dayjs.Dayjs>(dayjs().subtract(1, 'day'));
  const [compareDate, setCompareDate] = useState<dayjs.Dayjs>(dayjs().subtract(2, 'day'));
  const [intradayData, setIntradayData] = useState<any[]>([]);
  const [intradayLoading, setIntradayLoading] = useState(false);

  // -- daily trend --
  const [dailyPeriod, setDailyPeriod] = useState('week');
  const [dailyData, setDailyData] = useState<any[]>([]);
  const [dailyLoading, setDailyLoading] = useState(false);

  const powerMountedRef = useRef(true);
  const powerSeqRef = useRef(0);

  // Detect power metrics from telemetry data (runs once when building selected).
  // Uses ref for selectedMetric to avoid re-running when user switches metrics.
  const detectPowerMetrics = useCallback(async () => {
    if (!bid) return;
    const seq = ++powerSeqRef.current;
    try {
      const r = await api.get('/logs/telemetry', {
        params: {
          building_id: bid,
          interval: 'hour',
          start: dayjs().subtract(3, 'day').format('YYYY-MM-DD'),
          end: dayjs().format('YYYY-MM-DD'),
        },
      });
      if (seq !== powerSeqRef.current || !powerMountedRef.current) return;
      const rows: any[] = r.data || [];
      const metricSet = new Set<string>();
      rows.forEach((row: any) => {
        const m = String(row.metric || '').toLowerCase();
        if (POWER_KEYWORDS.some(kw => m.includes(kw))) {
          metricSet.add(row.metric);
        }
      });
      const list = Array.from(metricSet).sort();
      setPowerMetrics(list);
      if (!selectedMetricRef.current && list.length > 0) {
        setSelectedMetric(list[0]);
      }
    } catch { /* ignore */ }
  }, [bid]);

  useEffect(() => {
    powerMountedRef.current = true;
    if (subTab === 'power' && bid) detectPowerMetrics();
    return () => { powerMountedRef.current = false; };
  }, [subTab, bid, detectPowerMetrics]);

  // Fetch intraday comparison
  const fetchIntraday = useCallback(async () => {
    if (!bid) return;
    const seq = ++powerSeqRef.current;
    setIntradayLoading(true);
    try {
      const metric = selectedMetric || '';
      const baseStr = baseDate.format('YYYY-MM-DD');
      const cmpStr = compareDate.format('YYYY-MM-DD');

      const [baseRes, cmpRes] = await Promise.all([
        api.get('/logs/telemetry', {
          params: { building_id: bid, interval: 'hour', start: baseStr, end: baseStr + ' 23:59:59', ...(metric ? { metric } : {}) },
        }),
        api.get('/logs/telemetry', {
          params: { building_id: bid, interval: 'hour', start: cmpStr, end: cmpStr + ' 23:59:59', ...(metric ? { metric } : {}) },
        }),
      ]);

      if (seq !== powerSeqRef.current || !powerMountedRef.current) return;

      const baseMap: Record<number, number> = {};
      (baseRes.data || []).forEach((r: any) => {
        const h = dayjs(r.ts).hour();
        baseMap[h] = (baseMap[h] || 0) + (r.avg || r.value || 0);
      });

      const cmpMap: Record<number, number> = {};
      (cmpRes.data || []).forEach((r: any) => {
        const h = dayjs(r.ts).hour();
        cmpMap[h] = (cmpMap[h] || 0) + (r.avg || r.value || 0);
      });

      const rows: any[] = [];
      for (let h = 0; h < 24; h++) {
        rows.push({
          hour: String(h).padStart(2, '0') + ':00',
          [baseStr]: Number((baseMap[h] || 0).toFixed(2)),
          [cmpStr]: Number((cmpMap[h] || 0).toFixed(2)),
        });
      }
      setIntradayData(rows);
    } catch { if (seq === powerSeqRef.current && powerMountedRef.current) setIntradayData([]); }
    finally { if (seq === powerSeqRef.current && powerMountedRef.current) setIntradayLoading(false); }
  }, [bid, baseDate, compareDate, selectedMetric]);

  // Fetch daily trend
  const fetchDaily = useCallback(async () => {
    if (!bid) return;
    const seq = ++powerSeqRef.current;
    setDailyLoading(true);
    try {
      const metric = selectedMetric || '';
      const { start, interval } = periodToRange(dailyPeriod);
      const end = dayjs().format('YYYY-MM-DD');

      const r = await api.get('/logs/telemetry', {
        params: {
          building_id: bid, interval, start: start.format('YYYY-MM-DD'), end: end + ' 23:59:59',
          ...(metric ? { metric } : {}),
        },
      });

      if (seq !== powerSeqRef.current || !powerMountedRef.current) return;

      const rows = (r.data || []).map((row: any) => {
        let label: string;
        const ts = dayjs(row.ts);
        if (interval === 'hour') label = ts.format('MM-DD HH:00');
        else if (interval === 'month') label = ts.format('YYYY-MM');
        else label = ts.format('MM-DD');
        return { time: label, 用电量: Number((row.avg || row.value || 0).toFixed(2)) };
      });

      // Sort by time
      rows.sort((a: any, b: any) => a.time.localeCompare(b.time));

      setDailyData(rows);
    } catch { if (seq === powerSeqRef.current && powerMountedRef.current) setDailyData([]); }
    finally { if (seq === powerSeqRef.current && powerMountedRef.current) setDailyLoading(false); }
  }, [bid, dailyPeriod, selectedMetric]);

  // Auto-fetch charts when tab, building, or metric changes.
  // Dates and periods trigger manual refresh (via "刷新" button), not auto-fetch.
  useEffect(() => {
    if (subTab === 'power' && bid && selectedMetric) {
      fetchIntraday();
      fetchDaily();
    }
    // fetchIntraday/fetchDaily intentionally omitted: they change when dates/periods
    // change, which should NOT auto-trigger a refetch (user clicks "刷新" for those).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [subTab, bid, selectedMetric]);

  // ==================== FAULT STATS ====================
  const [faultPeriod, setFaultPeriod] = useState('week');
  const [faultTrendData, setFaultTrendData] = useState<any[]>([]);
  const [faultClassData, setFaultClassData] = useState<any[]>([]);
  const [faultLoading, setFaultLoading] = useState(false);
  const faultMountedRef = useRef(true);
  const faultSeqRef = useRef(0);

  const fetchFaults = useCallback(async () => {
    if (!bid) return;
    const seq = ++faultSeqRef.current;
    setFaultLoading(true);
    try {
      const { start } = periodToRange(faultPeriod);
      const end = dayjs().format('YYYY-MM-DD');

      const r = await api.get('/alarm-logs', {
        params: { building_id: bid, date_from: start.format('YYYY-MM-DD'), date_to: end + ' 23:59:59' },
      });

      if (seq !== faultSeqRef.current || !faultMountedRef.current) return;

      const logs: any[] = r.data || [];
      const interval = periodToAlarmInterval(faultPeriod);

      // -- Trend: group by time interval --
      const trendMap: Record<string, number> = {};
      logs.forEach((l: any) => {
        if (!l.created_at) return;
        let key: string;
        const ts = dayjs(l.created_at);
        if (interval === 'hour') key = ts.format('MM-DD HH:00');
        else if (interval === 'month') key = ts.format('YYYY-MM');
        else key = ts.format('MM-DD');
        trendMap[key] = (trendMap[key] || 0) + 1;
      });
      const trendRows = Object.entries(trendMap)
        .map(([time, count]) => ({ time, 故障数: count }))
        .sort((a, b) => a.time.localeCompare(b.time));

      // -- Classification: by device type --
      const classMap: Record<string, number> = {};
      logs.forEach((l: any) => {
        const dtype = deviceTypeMap[l.device_id] || '其他';
        classMap[dtype] = (classMap[dtype] || 0) + 1;
      });
      const classRows = Object.entries(classMap)
        .map(([type, count]) => ({ type, count }))
        .sort((a, b) => b.count - a.count);

      setFaultTrendData(trendRows);
      setFaultClassData(classRows);
    } catch {
      if (seq === faultSeqRef.current && faultMountedRef.current) {
        setFaultTrendData([]);
        setFaultClassData([]);
      }
    } finally {
      if (seq === faultSeqRef.current && faultMountedRef.current) setFaultLoading(false);
    }
  }, [bid, faultPeriod, deviceTypeMap]);

  useEffect(() => {
    faultMountedRef.current = true;
    if (subTab === 'fault' && bid) fetchFaults();
    return () => { faultMountedRef.current = false; };
  }, [subTab, bid, fetchFaults]);

  // ---- Dark chart theme props ----
  const chartGrid = { stroke: GRID_COLOR, strokeDasharray: '3 3' };
  const xAxisProps = { tick: { fill: AXIS_COLOR, fontSize: 11 }, axisLine: { stroke: GRID_COLOR }, tickLine: false };
  const yAxisProps = { tick: { fill: AXIS_COLOR, fontSize: 11 }, axisLine: { stroke: GRID_COLOR }, tickLine: false };
  const tooltipStyle = { contentStyle: { background: '#0d1f3c', border: '1px solid #1a3455', borderRadius: 4, color: '#c0d0e0' } };

  // ==================== Empty state ====================
  if (!bid) {
    return (
      <div style={{ padding: 16, height: 'calc(100vh - 112px)', overflowY: 'auto' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
          <ThunderboltOutlined style={{ color: '#00daf3', fontSize: 18 }} />
          <span style={{ fontSize: 16, fontWeight: 700, color: '#00daf3' }}>决策中心</span>
        </div>
        <div style={{ textAlign: 'center', padding: 80, color: '#5a7a9a' }}>
          <ThunderboltOutlined style={{ fontSize: 48, marginBottom: 12, display: 'block' }} />
          请先选择项目与楼宇
        </div>
      </div>
    );
  }

  return (
    <div style={{ padding: 16, height: 'calc(100vh - 112px)', overflowY: 'auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12 }}>
        <ThunderboltOutlined style={{ color: '#00daf3', fontSize: 18 }} />
        <span style={{ fontSize: 16, fontWeight: 700, color: '#00daf3' }}>决策中心</span>
      </div>

      {/* Sub-tabs */}
      <div style={{ display: 'flex', background: '#0d1f3c', borderRadius: 6, marginBottom: 16, border: '1px solid #1a3455', overflow: 'hidden' }}>
        {SUB_TABS.map(t => (
          <div key={t.key} onClick={() => setSubTab(t.key as any)} style={{
            padding: '8px 20px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 6,
            background: subTab === t.key ? '#152d50' : 'transparent',
            color: subTab === t.key ? '#00daf3' : '#8ba0c0',
            borderBottom: subTab === t.key ? '2px solid #00daf3' : '2px solid transparent',
            fontWeight: subTab === t.key ? 700 : 400, fontSize: 13,
          }}>{t.icon} {t.label}</div>
        ))}
      </div>

      {/* ==================== POWER STATS ==================== */}
      {subTab === 'power' && (
        <>
          {/* Metric selector */}
          {powerMetrics.length > 0 && (
            <div style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ color: '#8ba0c0', fontSize: 12 }}>电量指标:</span>
              {powerMetrics.map(m => (
                <Tag key={m} color={selectedMetric === m ? 'cyan' : undefined}
                  style={{ cursor: 'pointer', fontSize: 11 }}
                  onClick={() => setSelectedMetric(m)}>
                  {m}
                </Tag>
              ))}
            </div>
          )}
          {powerMetrics.length === 0 && !intradayLoading && (
            <div style={{ marginBottom: 12, color: '#5a7a9a', fontSize: 12 }}>
              未检测到电量指标，请确认设备已配置电量相关属性（如 power、energy、功率、电量 等）
            </div>
          )}

          {/* ---- 日内用电量走势对比 ---- */}
          <div style={darkCard}>
            <div style={sectionTitle}><ThunderboltOutlined /> 日内用电量走势对比</div>
            <div style={{ display: 'flex', gap: 16, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <span style={{ color: '#8ba0c0', fontSize: 12 }}>基准日:</span>
                <DatePicker size="small" value={baseDate} onChange={(v: any) => v && setBaseDate(v)}
                  style={{ background: '#0d1f3c', border: '1px solid #1a3455' }} format="YYYY-MM-DD" />
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <span style={{ color: '#8ba0c0', fontSize: 12 }}>对比日:</span>
                <DatePicker size="small" value={compareDate} onChange={(v: any) => v && setCompareDate(v)}
                  style={{ background: '#0d1f3c', border: '1px solid #1a3455' }} format="YYYY-MM-DD" />
              </div>
              <span style={{ color: '#5a7a9a', fontSize: 11, cursor: 'pointer' }} onClick={fetchIntraday}>刷新</span>
            </div>
            {intradayLoading ? (
              <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
            ) : intradayData.length === 0 ? (
              <div style={{ textAlign: 'center', padding: 40, color: '#5a7a9a' }}>暂无数据</div>
            ) : (
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={intradayData}>
                  <CartesianGrid {...chartGrid} />
                  <XAxis dataKey="hour" {...xAxisProps} />
                  <YAxis {...yAxisProps} label={{ value: '用电量(KWH)', angle: -90, position: 'insideLeft', fill: AXIS_COLOR, fontSize: 11, dx: -10 }} />
                  <Tooltip {...tooltipStyle} />
                  <Legend wrapperStyle={{ color: '#8ba0c0', fontSize: 12 }} />
                  <Line type="monotone" dataKey={baseDate.format('YYYY-MM-DD')} stroke={CYAN} strokeWidth={2} dot={false} name={`基准日 ${baseDate.format('MM-DD')}`} />
                  <Line type="monotone" dataKey={compareDate.format('YYYY-MM-DD')} stroke={ORANGE} strokeWidth={2} dot={false} name={`对比日 ${compareDate.format('MM-DD')} `} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>

          {/* ---- 日用电量走势 ---- */}
          <div style={{ ...darkCard, marginTop: 14 }}>
            <div style={sectionTitle}><ThunderboltOutlined /> 日用电量走势</div>
            <div style={{ display: 'flex', gap: 10, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }}>
              <Radio.Group value={dailyPeriod} onChange={e => setDailyPeriod(e.target.value)} size="small" optionType="button">
                {PERIODS.map(p => (
                  <Radio.Button key={p.value} value={p.value} style={{
                    background: dailyPeriod === p.value ? '#152d50' : '#0d1f3c',
                    borderColor: '#1a3455', color: dailyPeriod === p.value ? '#00daf3' : '#8ba0c0', fontSize: 11,
                  }}>{p.label}</Radio.Button>
                ))}
              </Radio.Group>
              <span style={{ color: '#5a7a9a', fontSize: 11, cursor: 'pointer' }} onClick={fetchDaily}>刷新</span>
            </div>
            {dailyLoading ? (
              <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
            ) : dailyData.length === 0 ? (
              <div style={{ textAlign: 'center', padding: 40, color: '#5a7a9a' }}>暂无数据</div>
            ) : (
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={dailyData}>
                  <CartesianGrid {...chartGrid} />
                  <XAxis dataKey="time" {...xAxisProps} />
                  <YAxis {...yAxisProps} label={{ value: '用电量(KWH)', angle: -90, position: 'insideLeft', fill: AXIS_COLOR, fontSize: 11, dx: -10 }} />
                  <Tooltip {...tooltipStyle} />
                  <Line type="monotone" dataKey="用电量" stroke={CYAN} strokeWidth={2} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>
        </>
      )}

      {/* ==================== FAULT STATS ==================== */}
      {subTab === 'fault' && (
        <>
          {/* ---- 日故障走势 ---- */}
          <div style={darkCard}>
            <div style={sectionTitle}><AlertOutlined /> 日故障走势</div>
            <div style={{ display: 'flex', gap: 10, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }}>
              <Radio.Group value={faultPeriod} onChange={e => setFaultPeriod(e.target.value)} size="small" optionType="button">
                {PERIODS.map(p => (
                  <Radio.Button key={p.value} value={p.value} style={{
                    background: faultPeriod === p.value ? '#152d50' : '#0d1f3c',
                    borderColor: '#1a3455', color: faultPeriod === p.value ? '#00daf3' : '#8ba0c0', fontSize: 11,
                  }}>{p.label}</Radio.Button>
                ))}
              </Radio.Group>
              <span style={{ color: '#5a7a9a', fontSize: 11, cursor: 'pointer' }} onClick={fetchFaults}>刷新</span>
            </div>
            {faultLoading ? (
              <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
            ) : faultTrendData.length === 0 ? (
              <div style={{ textAlign: 'center', padding: 40, color: '#5a7a9a' }}>暂无故障记录</div>
            ) : (
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={faultTrendData}>
                  <CartesianGrid {...chartGrid} />
                  <XAxis dataKey="time" {...xAxisProps} />
                  <YAxis {...yAxisProps} label={{ value: '故障数', angle: -90, position: 'insideLeft', fill: AXIS_COLOR, fontSize: 11, dx: -10 }} allowDecimals={false} />
                  <Tooltip {...tooltipStyle} />
                  <Line type="monotone" dataKey="故障数" stroke={RED} strokeWidth={2} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            )}
          </div>

          {/* ---- 设备故障分类统计 ---- */}
          <div style={{ ...darkCard, marginTop: 14 }}>
            <div style={sectionTitle}><AlertOutlined /> 设备故障分类统计</div>
            <div style={{ display: 'flex', gap: 10, marginBottom: 16, alignItems: 'center', flexWrap: 'wrap' }}>
              <Radio.Group value={faultPeriod} onChange={e => setFaultPeriod(e.target.value)} size="small" optionType="button">
                {PERIODS.map(p => (
                  <Radio.Button key={p.value} value={p.value} style={{
                    background: faultPeriod === p.value ? '#152d50' : '#0d1f3c',
                    borderColor: '#1a3455', color: faultPeriod === p.value ? '#00daf3' : '#8ba0c0', fontSize: 11,
                  }}>{p.label}</Radio.Button>
                ))}
              </Radio.Group>
              <span style={{ color: '#5a7a9a', fontSize: 11, cursor: 'pointer' }} onClick={fetchFaults}>刷新</span>
            </div>
            {faultLoading ? (
              <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
            ) : faultClassData.length === 0 ? (
              <div style={{ textAlign: 'center', padding: 40, color: '#5a7a9a' }}>暂无故障记录</div>
            ) : (
              <ResponsiveContainer width="100%" height={300}>
                <BarChart data={faultClassData}>
                  <CartesianGrid {...chartGrid} />
                  <XAxis dataKey="type" {...xAxisProps} />
                  <YAxis {...yAxisProps} label={{ value: '故障数', angle: -90, position: 'insideLeft', fill: AXIS_COLOR, fontSize: 11, dx: -10 }} allowDecimals={false} />
                  <Tooltip {...tooltipStyle} />
                  <Bar dataKey="count" fill={ORANGE} name="故障数" radius={[3, 3, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </>
      )}
    </div>
  );
}
