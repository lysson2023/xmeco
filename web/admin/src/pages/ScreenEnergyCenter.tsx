import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { Spin, Tag } from 'antd';
import { ThunderboltOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import {
  LineChart, Line, BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip, Legend, ResponsiveContainer,
} from 'recharts';
import { api } from '../api/screenClient';

// ---- dark theme chart colors ----
const CYAN = '#00daf3';
const ORANGE = '#fa8c16';
const GREEN = '#52c41a';
const AXIS_COLOR = '#5a7a9a';
const GRID_COLOR = '#1a3455';
const METER_LINE_COLORS = ['#00daf3', '#fa8c16', '#52c41a', '#ff4d4f', '#722ed1', '#13c2c2', '#eb2f96', '#faad14', '#2f54eb', '#a0d911', '#f759ab', '#597ef7'];

const POWER_KEYWORDS = ['power', 'energy', 'kwh', '功率', '电量', '电度', '有功', '电能'];

const darkCard: React.CSSProperties = {
  background: '#0d1f3c', borderRadius: 6, padding: 16, border: '1px solid #1a3455',
};

const sectionTitle: React.CSSProperties = {
  fontSize: 14, fontWeight: 700, color: '#00daf3', marginBottom: 12,
  display: 'flex', alignItems: 'center', gap: 6,
};

const statBox: React.CSSProperties = {
  background: '#0a1628', borderRadius: 4, padding: '10px 14px', border: '1px solid #1a3455',
  textAlign: 'center', minWidth: 110,
};

// ---- helpers ----

/** Sum energy from telemetry aggregated rows. Each row.avg is average value over the interval.
 *  Energy (kWh) = avg × intervalHours for power metrics.
 */
function sumEnergy(rows: any[], intervalHours: number): number {
  return rows.reduce((s: number, r: any) => s + (Number(r.avg) || 0) * intervalHours, 0);
}

function fmtNum(n: number): string {
  if (Math.abs(n) >= 10000) return (n / 10000).toFixed(1) + '万';
  if (Math.abs(n) >= 1000) return n.toFixed(0);
  return n.toFixed(2);
}

const WEEKDAY_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];

interface ScreenEnergyCenterProps {
  bid: number;
  devices: any[];
  meterPower: number;
}

export default function ScreenEnergyCenter({ bid, devices, meterPower }: ScreenEnergyCenterProps) {
  // ---- memoized meter list ----
  const meters = useMemo(() => devices.filter((d: any) => d.type === '电表'), [devices]);
  const meterCount = meters.length;

  // ---- metric selection ----
  const [powerMetrics, setPowerMetrics] = useState<string[]>([]);
  const [selectedMetric, setSelectedMetric] = useState('');
  const selectedMetricRef = useRef(selectedMetric);
  selectedMetricRef.current = selectedMetric;

  // ---- overview stats ----
  const [stats, setStats] = useState<Record<string, number>>({});
  const [statsLoading, setStatsLoading] = useState(false);

  // ---- chart data ----
  const [hourlyData, setHourlyData] = useState<any[]>([]);
  const [hourlyLoading, setHourlyLoading] = useState(false);
  const [weeklyData, setWeeklyData] = useState<any[]>([]);
  const [weeklyLoading, setWeeklyLoading] = useState(false);
  const [monthlyCompData, setMonthlyCompData] = useState<any[]>([]);
  const [monthlyCompLoading, setMonthlyCompLoading] = useState(false);
  const [yearlyCompData, setYearlyCompData] = useState<any[]>([]);
  const [yearlyCompLoading, setYearlyCompLoading] = useState(false);

  // ---- refs for race condition prevention (detect 和 fetch 使用独立序号) ----
  const mountedRef = useRef(true);
  const detectSeqRef = useRef(0);
  const fetchSeqRef = useRef(0);

  // ---- detect power metrics ----
  const detectMetrics = useCallback(async () => {
    if (!bid) return;
    const seq = ++detectSeqRef.current;
    try {
      const r = await api.get('/logs/telemetry', {
        params: {
          building_id: bid,
          interval: 'hour',
          start: dayjs().subtract(3, 'day').format('YYYY-MM-DD'),
          end: dayjs().format('YYYY-MM-DD'),
        },
      });
      if (seq !== detectSeqRef.current || !mountedRef.current) return;
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

  // ---- fetch all data (stats + charts) ----
  const fetchAll = useCallback(async () => {
    if (!bid || !selectedMetric) return;
    const seq = ++fetchSeqRef.current;

    setStatsLoading(true);
    setHourlyLoading(true);
    setWeeklyLoading(true);
    setMonthlyCompLoading(true);
    setYearlyCompLoading(true);

    const now = dayjs();
    const yesterday = now.subtract(1, 'day');
    const dayBefore = now.subtract(2, 'day');
    // last week: Mon-Sun of previous week
    const lastMonday = now.subtract(1, 'week').startOf('week').add(1, 'day'); // Monday
    const lastSunday = lastMonday.add(6, 'day');
    // this week: Monday of this week to now
    const thisMonday = now.startOf('week').add(1, 'day');
    // last month
    const lastMonth1st = now.subtract(1, 'month').startOf('month');
    const lastMonthLast = now.subtract(1, 'month').endOf('month');
    // this month
    const thisMonth1st = now.startOf('month');
    // last year same month
    const lastYearThisMonth1st = now.subtract(1, 'year').startOf('month');
    const lastYearThisMonthLast = now.subtract(1, 'year').endOf('month');
    // last year
    const lastYearJan = dayjs().subtract(1, 'year').startOf('year');
    // this year
    const thisYearJan = dayjs().startOf('year');

    const metric = selectedMetric;
    const fmt = 'YYYY-MM-DD';
    const fmtEnd = 'YYYY-MM-DD HH:mm:ss';

    try {
      const [
        // A: yesterday hourly (for chart)
        hourlyRes,
        // B: day-before + yesterday stats
        twoDayRes,
        // C: last week + this week stats + last week chart
        weekRangeRes,
        // D: last month + this month stats + this month chart
        monthRangeRes,
        // E: last year same month chart
        lastYearMonthRes,
        // F: yearly stats + chart
        yearRangeRes,
        // Per-meter hourly queries for yesterday
        ...meterResults
      ] = await Promise.all([
        // A
        api.get('/logs/telemetry', {
          params: { building_id: bid, interval: 'hour', metric, start: yesterday.format(fmt), end: yesterday.format(fmt) + ' 23:59:59' },
        }),
        // B
        api.get('/logs/telemetry', {
          params: { building_id: bid, interval: 'day', metric, start: dayBefore.format(fmt), end: yesterday.format(fmt) + ' 23:59:59' },
        }),
        // C
        api.get('/logs/telemetry', {
          params: { building_id: bid, interval: 'day', metric, start: lastMonday.format(fmt), end: now.format(fmtEnd) },
        }),
        // D
        api.get('/logs/telemetry', {
          params: { building_id: bid, interval: 'day', metric, start: lastMonth1st.format(fmt), end: now.format(fmtEnd) },
        }),
        // E
        api.get('/logs/telemetry', {
          params: { building_id: bid, interval: 'day', metric, start: lastYearThisMonth1st.format(fmt), end: lastYearThisMonthLast.format(fmt) + ' 23:59:59' },
        }),
        // F
        api.get('/logs/telemetry', {
          params: { building_id: bid, interval: 'month', metric, start: lastYearJan.format(fmt), end: now.format(fmtEnd) },
        }),
        // Per-meter hourly queries
        ...meters.map(m =>
          api.get('/logs/telemetry', {
            params: { device_id: m.id, interval: 'hour', metric, start: yesterday.format(fmt), end: yesterday.format(fmt) + ' 23:59:59' },
          })
        ),
      ]);

      if (seq !== fetchSeqRef.current || !mountedRef.current) return;

      // ---- Process stats ----
      const dayBeforeRows = (twoDayRes.data || []).filter((r: any) => dayjs(r.ts).format(fmt) === dayBefore.format(fmt));
      const yesterdayRows = (twoDayRes.data || []).filter((r: any) => dayjs(r.ts).format(fmt) === yesterday.format(fmt));

      const weekRowsAll: any[] = weekRangeRes.data || [];
      const lastWeekRows = weekRowsAll.filter((r: any) => {
        const d = dayjs(r.ts);
        return d.isAfter(lastMonday.subtract(1, 'day')) && d.isBefore(lastSunday.add(1, 'day'));
      });
      const thisWeekRows = weekRowsAll.filter((r: any) => {
        const d = dayjs(r.ts);
        return d.isAfter(thisMonday.subtract(1, 'day')) && d.isBefore(now.add(1, 'day'));
      });

      const monthRowsAll: any[] = monthRangeRes.data || [];
      const lastMonthRows = monthRowsAll.filter((r: any) => {
        const d = dayjs(r.ts);
        return d.month() === lastMonth1st.month() && d.year() === lastMonth1st.year();
      });
      const thisMonthRows = monthRowsAll.filter((r: any) => {
        const d = dayjs(r.ts);
        return d.month() === thisMonth1st.month() && d.year() === thisMonth1st.year();
      });

      const yearRowsAll: any[] = yearRangeRes.data || [];
      const lastYearRows = yearRowsAll.filter((r: any) => dayjs(r.ts).year() === lastYearJan.year());
      const thisYearRows = yearRowsAll.filter((r: any) => dayjs(r.ts).year() === thisYearJan.year());

      // 年度统计：按月分组后逐月计算（每月天数不同），避免用单月天数 × 全年数据
      const sumEnergyByMonth = (rows: any[], year: number): number => {
        const byMonth: Record<number, any[]> = {};
        rows.forEach((r: any) => {
          const m = dayjs(r.ts).month();
          if (!byMonth[m]) byMonth[m] = [];
          byMonth[m].push(r);
        });
        let total = 0;
        for (let m = 0; m < 12; m++) {
          if (byMonth[m]) {
            total += sumEnergy(byMonth[m], dayjs().year(year).month(m).daysInMonth() * 24);
          }
        }
        return total;
      };

      setStats({
        meterCount,
        yesterday: sumEnergy(yesterdayRows, 24),
        dayBefore: sumEnergy(dayBeforeRows, 24),
        lastWeek: sumEnergy(lastWeekRows, 24),
        thisWeek: sumEnergy(thisWeekRows, 24),
        lastMonth: sumEnergy(lastMonthRows, 24),
        thisMonth: sumEnergy(thisMonthRows, 24),
        lastYear: sumEnergyByMonth(lastYearRows, lastYearJan.year()),
        thisYear: sumEnergyByMonth(thisYearRows, thisYearJan.year()),
      });

      // ---- Process yesterday hourly chart ----
      // Total data
      const totalMap: Record<number, number> = {};
      (hourlyRes.data || []).forEach((r: any) => {
        const h = dayjs(r.ts).hour();
        totalMap[h] = (totalMap[h] || 0) + (Number(r.avg) || 0);
      });

      // Per-meter data
      const meterMaps: Record<number, Record<number, number>> = {};
      meters.forEach((m, i) => {
        const map: Record<number, number> = {};
        (meterResults[i]?.data || []).forEach((r: any) => {
          const h = dayjs(r.ts).hour();
          map[h] = (map[h] || 0) + (Number(r.avg) || 0);
        });
        meterMaps[m.id] = map;
      });

      const hourlyRows: any[] = [];
      for (let h = 0; h < 24; h++) {
        const row: any = { hour: String(h).padStart(2, '0') + ':00' };
        row['总计'] = Number((totalMap[h] || 0).toFixed(2));
        meters.forEach(m => {
          row[m.name] = Number(((meterMaps[m.id] || {})[h] || 0).toFixed(2));
        });
        hourlyRows.push(row);
      }
      setHourlyData(hourlyRows);

      // ---- Process last week daily chart (Mon-Sun) ----
      const weekChartRows: any[] = [];
      for (let d = 0; d < 7; d++) {
        const day = lastMonday.add(d, 'day');
        const dayStr = day.format(fmt);
        const dayRows = lastWeekRows.filter((r: any) => dayjs(r.ts).format(fmt) === dayStr);
        weekChartRows.push({
          day: WEEKDAY_NAMES[day.day()],
          用电量: Number(sumEnergy(dayRows, 24).toFixed(2)),
        });
      }
      setWeeklyData(weekChartRows);

      // ---- Process monthly comparison chart ----
      const lastYearMonthRows: any[] = lastYearMonthRes.data || [];
      const daysInThisMonth = thisMonth1st.daysInMonth();
      const monthlyCompRows: any[] = [];
      for (let d = 1; d <= daysInThisMonth; d++) {
        const thisDay = thisMonth1st.date(d);
        const thisDayStr = thisDay.format(fmt);
        const lastYearDay = lastYearThisMonth1st.date(Math.min(d, lastYearThisMonth1st.daysInMonth()));
        const lastYearDayStr = lastYearDay.format(fmt);

        const thisDayRows = thisMonthRows.filter((r: any) => dayjs(r.ts).format(fmt) === thisDayStr);
        const lastYearDayRows = lastYearMonthRows.filter((r: any) => dayjs(r.ts).format(fmt) === lastYearDayStr);

        // Only include days up to today for this month
        if (thisDay.isAfter(now)) break;

        monthlyCompRows.push({
          day: d + '日',
          '本月': Number(sumEnergy(thisDayRows, 24).toFixed(2)),
          '去年同月': Number(sumEnergy(lastYearDayRows, 24).toFixed(2)),
        });
      }
      setMonthlyCompData(monthlyCompRows);

      // ---- Process yearly comparison chart ----
      const yearlyCompRows: any[] = [];
      for (let m = 0; m < 12; m++) {
        const thisYearMonth = thisYearJan.month(m);
        const lastYearMonth = lastYearJan.month(m);

        const thisMonthYearRows = thisYearRows.filter((r: any) => dayjs(r.ts).month() === m);
        const lastMonthYearRows = lastYearRows.filter((r: any) => dayjs(r.ts).month() === m);

        // Only include months up to current month for this year
        if (thisYearMonth.isAfter(now, 'month')) break;

        yearlyCompRows.push({
          month: (m + 1) + '月',
          '今年': Number(sumEnergy(thisMonthYearRows, thisYearMonth.daysInMonth() * 24).toFixed(2)),
          '去年': Number(sumEnergy(lastMonthYearRows, lastYearMonth.daysInMonth() * 24).toFixed(2)),
        });
      }
      setYearlyCompData(yearlyCompRows);

    } catch {
      if (seq === fetchSeqRef.current && mountedRef.current) {
        setStats({});
        setHourlyData([]);
        setWeeklyData([]);
        setMonthlyCompData([]);
        setYearlyCompData([]);
      }
    } finally {
      if (seq === fetchSeqRef.current && mountedRef.current) {
        setStatsLoading(false);
        setHourlyLoading(false);
        setWeeklyLoading(false);
        setMonthlyCompLoading(false);
        setYearlyCompLoading(false);
      }
    }
  }, [bid, selectedMetric]); // eslint-disable-line react-hooks/exhaustive-deps

  // ---- lifecycle ----
  useEffect(() => {
    mountedRef.current = true;
    if (bid) detectMetrics();
    return () => { mountedRef.current = false; };
  }, [bid, detectMetrics]);

  useEffect(() => {
    if (bid && selectedMetric) fetchAll();
  }, [bid, selectedMetric, fetchAll]);

  // ---- dark chart theme props (module-level constants for referential stability) ----
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
          <span style={{ fontSize: 16, fontWeight: 700, color: '#00daf3' }}>能耗中心</span>
        </div>
        <div style={{ textAlign: 'center', padding: 80, color: '#5a7a9a' }}>
          <ThunderboltOutlined style={{ fontSize: 48, marginBottom: 12, display: 'block' }} />
          请先选择项目与楼宇
        </div>
      </div>
    );
  }

  // ==================== Main render ====================
  return (
    <div style={{ padding: 16, height: 'calc(100vh - 112px)', overflowY: 'auto' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
        <ThunderboltOutlined style={{ color: '#00daf3', fontSize: 18 }} />
        <span style={{ fontSize: 16, fontWeight: 700, color: '#00daf3' }}>能耗中心</span>
      </div>

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
      {powerMetrics.length === 0 && !statsLoading && (
        <div style={{ marginBottom: 12, color: '#5a7a9a', fontSize: 12 }}>
          未检测到电量指标，请确认设备已配置电量相关属性
        </div>
      )}

      {/* ==================== Overview Stats ==================== */}
      <div style={darkCard}>
        <div style={sectionTitle}><ThunderboltOutlined /> 设备总览</div>
        {statsLoading && Object.keys(stats).length === 0 ? (
          <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
        ) : (
          <>
            {/* Row 1 */}
            <div style={{ display: 'flex', gap: 8, marginBottom: 8, flexWrap: 'wrap' }}>
              <div style={statBox}>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>电表总数</div>
                <div style={{ fontSize: 22, fontWeight: 700, color: CYAN }}>{meterCount}</div>
                <div style={{ fontSize: 10, color: '#5a7a9a' }}>台</div>
              </div>
              <div style={statBox}>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>用电总数</div>
                <div style={{ fontSize: 22, fontWeight: 700, color: CYAN }}>{fmtNum(meterPower)}</div>
                <div style={{ fontSize: 10, color: '#5a7a9a' }}>当前 kW</div>
              </div>
              <div style={statBox}>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>昨天</div>
                <div style={{ fontSize: 22, fontWeight: 700, color: '#c0d0e0' }}>{fmtNum(stats.yesterday || 0)}</div>
                <div style={{ fontSize: 10, color: '#5a7a9a' }}>kWh</div>
              </div>
              <div style={statBox}>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>前天</div>
                <div style={{ fontSize: 22, fontWeight: 700, color: '#c0d0e0' }}>{fmtNum(stats.dayBefore || 0)}</div>
                <div style={{ fontSize: 10, color: '#5a7a9a' }}>kWh</div>
              </div>
              <div style={statBox}>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>上星期</div>
                <div style={{ fontSize: 22, fontWeight: 700, color: '#c0d0e0' }}>{fmtNum(stats.lastWeek || 0)}</div>
                <div style={{ fontSize: 10, color: '#5a7a9a' }}>kWh</div>
              </div>
              <div style={statBox}>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>本星期</div>
                <div style={{ fontSize: 22, fontWeight: 700, color: ORANGE }}>{fmtNum(stats.thisWeek || 0)}</div>
                <div style={{ fontSize: 10, color: '#5a7a9a' }}>截止上一小时 kWh</div>
              </div>
            </div>
            {/* Row 2 */}
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              <div style={statBox}>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>上月</div>
                <div style={{ fontSize: 22, fontWeight: 700, color: '#c0d0e0' }}>{fmtNum(stats.lastMonth || 0)}</div>
                <div style={{ fontSize: 10, color: '#5a7a9a' }}>kWh</div>
              </div>
              <div style={statBox}>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>本月</div>
                <div style={{ fontSize: 22, fontWeight: 700, color: ORANGE }}>{fmtNum(stats.thisMonth || 0)}</div>
                <div style={{ fontSize: 10, color: '#5a7a9a' }}>截止上一小时 kWh</div>
              </div>
              <div style={statBox}>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>去年</div>
                <div style={{ fontSize: 22, fontWeight: 700, color: '#c0d0e0' }}>{fmtNum(stats.lastYear || 0)}</div>
                <div style={{ fontSize: 10, color: '#5a7a9a' }}>kWh</div>
              </div>
              <div style={statBox}>
                <div style={{ fontSize: 11, color: '#8ba0c0' }}>今年</div>
                <div style={{ fontSize: 22, fontWeight: 700, color: ORANGE }}>{fmtNum(stats.thisYear || 0)}</div>
                <div style={{ fontSize: 10, color: '#5a7a9a' }}>截止上一小时 kWh</div>
              </div>
            </div>
          </>
        )}
      </div>

      {/* ==================== Yesterday 24-Hour Chart ==================== */}
      <div style={{ ...darkCard, marginTop: 14 }}>
        <div style={sectionTitle}><ThunderboltOutlined /> 昨日24小时用能走势</div>
        {hourlyLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
        ) : hourlyData.length === 0 ? (
          <div style={{ textAlign: 'center', padding: 40, color: '#5a7a9a' }}>暂无数据</div>
        ) : (
          <ResponsiveContainer width="100%" height={320}>
            <LineChart data={hourlyData}>
              <CartesianGrid {...chartGrid} />
              <XAxis dataKey="hour" {...xAxisProps} />
              <YAxis {...yAxisProps} label={{ value: '用电量(kWh)', angle: -90, position: 'insideLeft', fill: AXIS_COLOR, fontSize: 11, dx: -10 }} />
              <Tooltip {...tooltipStyle} />
              <Legend wrapperStyle={{ color: '#8ba0c0', fontSize: 11 }} />
              {/* Total line - thick & prominent */}
              <Line type="monotone" dataKey="总计" stroke={CYAN} strokeWidth={2.5} dot={false} name="总计" />
              {/* Per-meter lines */}
              {meters.map((m, i) => (
                <Line key={m.id} type="monotone" dataKey={m.name}
                  stroke={METER_LINE_COLORS[i % METER_LINE_COLORS.length]}
                  strokeWidth={1.2} dot={false} strokeDasharray={i === 0 ? undefined : '4 2'}
                  name={m.name} />
              ))}
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* ==================== Row: Last Week + Yearly side by side ==================== */}
      <div style={{ display: 'flex', gap: 14, marginTop: 14 }}>
        {/* ---- Last Week Daily Chart ---- */}
        <div style={{ ...darkCard, flex: 1, minWidth: 0 }}>
          <div style={sectionTitle}><ThunderboltOutlined /> 上星期每日用能趋势</div>
          {weeklyLoading ? (
            <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
          ) : weeklyData.length === 0 ? (
            <div style={{ textAlign: 'center', padding: 40, color: '#5a7a9a' }}>暂无数据</div>
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={weeklyData}>
                <CartesianGrid {...chartGrid} />
                <XAxis dataKey="day" {...xAxisProps} />
                <YAxis {...yAxisProps} />
                <Tooltip {...tooltipStyle} />
                <Bar dataKey="用电量" fill={CYAN} radius={[3, 3, 0, 0]} name="用电量" />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>

        {/* ---- Yearly Comparison Chart ---- */}
        <div style={{ ...darkCard, flex: 1, minWidth: 0 }}>
          <div style={sectionTitle}><ThunderboltOutlined /> 年度12月份用能趋势</div>
          <div style={{ fontSize: 11, color: '#5a7a9a', marginBottom: 8, marginTop: -8 }}>
            {dayjs().format('YYYY年')} vs {dayjs().subtract(1, 'year').format('YYYY年')} 每月对比
          </div>
          {yearlyCompLoading ? (
            <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
          ) : yearlyCompData.length === 0 ? (
            <div style={{ textAlign: 'center', padding: 40, color: '#5a7a9a' }}>暂无数据</div>
          ) : (
            <ResponsiveContainer width="100%" height={280}>
              <BarChart data={yearlyCompData}>
                <CartesianGrid {...chartGrid} />
                <XAxis dataKey="month" {...xAxisProps} />
                <YAxis {...yAxisProps} />
                <Tooltip {...tooltipStyle} />
                <Legend wrapperStyle={{ color: '#8ba0c0', fontSize: 11 }} />
                <Bar dataKey="今年" fill={CYAN} radius={[3, 3, 0, 0]} name="今年" />
                <Bar dataKey="去年" fill={ORANGE} radius={[3, 3, 0, 0]} name="去年" />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* ==================== Monthly Comparison Chart ==================== */}
      <div style={{ ...darkCard, marginTop: 14 }}>
        <div style={sectionTitle}><ThunderboltOutlined /> 月能耗用能趋势</div>
        <div style={{ fontSize: 11, color: '#5a7a9a', marginBottom: 8, marginTop: -8 }}>
          {dayjs().format('YYYY年M月')} vs {dayjs().subtract(1, 'year').format('YYYY年M月')} 每日对比
        </div>
        {monthlyCompLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
        ) : monthlyCompData.length === 0 ? (
          <div style={{ textAlign: 'center', padding: 40, color: '#5a7a9a' }}>暂无数据</div>
        ) : (
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={monthlyCompData}>
              <CartesianGrid {...chartGrid} />
              <XAxis dataKey="day" {...xAxisProps} interval={Math.max(0, Math.floor(monthlyCompData.length / 15) - 1)} />
              <YAxis {...yAxisProps} label={{ value: '用电量(kWh)', angle: -90, position: 'insideLeft', fill: AXIS_COLOR, fontSize: 11, dx: -10 }} />
              <Tooltip {...tooltipStyle} />
              <Legend wrapperStyle={{ color: '#8ba0c0', fontSize: 11 }} />
              <Bar dataKey="本月" fill={CYAN} radius={[3, 3, 0, 0]} name="本月" />
              <Bar dataKey="去年同月" fill={ORANGE} radius={[3, 3, 0, 0]} name="去年同月" />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>

    </div>
  );
}
