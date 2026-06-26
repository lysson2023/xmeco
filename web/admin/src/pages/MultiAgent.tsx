import { useEffect, useState } from 'react';
import { Card, Row, Col, Table, Tag, Spin, Alert, Tabs, message, Select, DatePicker, Button, Descriptions, Progress, Statistic, Divider, Space } from 'antd';
import {
  ThunderboltOutlined, LineChartOutlined, SettingOutlined,
  RobotOutlined, BulbOutlined, ControlOutlined, DollarOutlined, SyncOutlined,
  DashboardOutlined, DownloadOutlined, SearchOutlined,
} from '@ant-design/icons';
import api from '../api/client';
import type { Dayjs } from 'dayjs';
import type { EfficiencyItem, MeterInfo } from '../types';

const { RangePicker } = DatePicker;

// ---- Types ----
interface ForecastItem { hour: number; temp: number; load_kw: number; load_pct: number; }
interface Recommendation {
  device_name: string; parameter: string;
  current_value: number; recommended_value: number; unit: string;
  reason: string; estimated_save_kw: number; priority: string;
}
interface Linkage {
  device_name: string; current_cooling_temp: number; target_cooling_temp: number;
  wet_bulb_temp: number; cop_improvement: number; save_kw_per_hour: number;
  reason: string; active: boolean;
}
interface PumpOpt {
  device_name: string; device_type: string;
  current_freq: number; target_freq: number; save_kw_per_hour: number;
  reason: string; active: boolean; delta_t: number;
}
interface PriceTactic {
  current_hour: number; current_period: string; current_price: number;
  recommendation: string; periods: { name: string; start: number; end: number; price: number }[];
}
interface RotationItem {
  device_name: string; device_type: string; run_hours: number;
  recommendation: string; reason: string; priority: number;
}

// ---- Power Quality Types ----
interface VoltageQuality { avg_v: number; min_v: number; max_v: number; nominal_v: number; deviation_pct: number; qualified_rate: number; samples: number; grade: string; }
interface CurrentBalance { avg_a: number; avg_b: number; avg_c: number; imbalance_pct: number; max_phase_current: number; samples: number; grade: string; }
interface PowerFactorQ { avg_pf: number; min_pf: number; max_pf: number; qualified_rate: number; samples: number; grade: string; }
interface FreqQuality { avg_hz: number; min_hz: number; max_hz: number; nominal_hz: number; deviation_pct: number; qualified_rate: number; samples: number; grade: string; }
interface HarmonicsQ { avg_thd_v: number; max_thd_v: number; avg_thd_i: number; max_thd_i: number; samples: number; grade: string; }
interface PqResult {
  device_name: string; device_type: string; time_range: string;
  voltage: VoltageQuality | null; current: CurrentBalance | null;
  power_factor: PowerFactorQ | null; frequency: FreqQuality | null;
  harmonics: HarmonicsQ | null; summary: string; overall_grade: string;
}

export default function MultiAgent() {
  const [loading, setLoading] = useState(true);
  const [efficiencies, setEfficiencies] = useState<EfficiencyItem[]>([]);
  const [forecast, setForecast] = useState<ForecastItem[]>([]);
  const [recommendations, setRecommendations] = useState<Recommendation[]>([]);
  const [summary, setSummary] = useState('');

  // Strategies
  const [stratLoading, setStratLoading] = useState(true);
  const [linkages, setLinkages] = useState<Linkage[]>([]);
  const [pumps, setPumps] = useState<PumpOpt[]>([]);
  const [priceTactic, setPriceTactic] = useState<PriceTactic | null>(null);
  const [rotation, setRotation] = useState<RotationItem[]>([]);
  const [stratSummary, setStratSummary] = useState('');
  const [totalSave, setTotalSave] = useState(0);

  // Power Quality
  const [pqProjects, setPqProjects] = useState<any[]>([]);
  const [pqBuildings, setPqBuildings] = useState<any[]>([]);
  const [meters, setMeters] = useState<MeterInfo[]>([]);
  const [selectedProject, setSelectedProject] = useState<number | null>(null);
  const [selectedBuilding, setSelectedBuilding] = useState<number | null>(null);
  const [selectedMeter, setSelectedMeter] = useState<number | null>(null);
  const [pqResult, setPqResult] = useState<PqResult | null>(null);
  const [pqLoading, setPqLoading] = useState(false);
  const [pqTimeRange, setPqTimeRange] = useState<[Dayjs, Dayjs] | null>(null);

  useEffect(() => {
    setLoading(true);
    setStratLoading(true);
    Promise.all([
      api.get('/intelligence/full'),
      api.get('/intelligence/strategies'),
    ]).then(([analysis, strat]) => {
      setEfficiencies(analysis.data.efficiencies || []);
      setForecast(analysis.data.forecast || []);
      setRecommendations(analysis.data.recommendations || []);
      setSummary(analysis.data.summary || '');

      setLinkages(strat.data.linkages || []);
      setPumps(strat.data.pump_optimize || []);
      setPriceTactic(strat.data.price_tactic);
      setRotation(strat.data.rotation_plan || []);
      setStratSummary(strat.data.summary || '');
      setTotalSave(strat.data.total_save_kw || 0);
    }).catch(() => {
      message.error('智能分析加载失败，请稍后重试');
    }).finally(() => { setLoading(false); setStratLoading(false); });
  }, []);

  // Load projects & buildings for PQ cascading
  useEffect(() => {
    api.get('/projects').then(r => setPqProjects(r.data || [])).catch(() => {});
    api.get('/buildings').then(r => setPqBuildings(r.data || [])).catch(() => {});
  }, []);

  // Load meter devices when building changes
  useEffect(() => {
    if (selectedBuilding) {
      api.get('/intelligence/meter-devices', { params: { building_id: selectedBuilding } })
        .then(r => setMeters(r.data || [])).catch(() => setMeters([]));
      setSelectedMeter(null); setPqResult(null);
    } else { setMeters([]); setSelectedMeter(null); }
  }, [selectedBuilding]);

  // Analyze power quality
  const analyzePQ = async () => {
    if (!selectedMeter) { message.warning('请先选择电表'); return; }
    setPqLoading(true);
    try {
      const params: any = { device_id: selectedMeter };
      if (pqTimeRange) {
        params.start = pqTimeRange[0].toISOString();
        params.end = pqTimeRange[1].toISOString();
      }
      const { data } = await api.get('/intelligence/power-quality', { params });
      setPqResult(data);
    } catch {
      message.error('电能质量分析失败');
    } finally { setPqLoading(false); }
  };

  // Export PQ result as CSV
  const exportPQ = () => {
    if (!pqResult) return;
    const rows: string[] = ['指标,数值,评级'];
    const add = (label: string, val: string, grade: string) => rows.push(`${label},${val},${grade}`);
    if (pqResult.voltage) {
      const v = pqResult.voltage;
      add('平均电压', `${v.avg_v}V`, v.grade);
      add('电压偏差', `${v.deviation_pct}%`, v.grade);
      add('合格率', `${v.qualified_rate}%`, v.grade);
    }
    if (pqResult.current) {
      const c = pqResult.current;
      add('三相不平衡度', `${c.imbalance_pct}%`, c.grade);
      add('A相平均电流', `${c.avg_a}A`, '');
      add('B相平均电流', `${c.avg_b}A`, '');
      add('C相平均电流', `${c.avg_c}A`, '');
    }
    if (pqResult.power_factor) {
      const p = pqResult.power_factor;
      add('平均功率因数', `${p.avg_pf}`, p.grade);
      add('功率因数合格率', `${p.qualified_rate}%`, p.grade);
    }
    if (pqResult.frequency) {
      const f = pqResult.frequency;
      add('平均频率', `${f.avg_hz}Hz`, f.grade);
      add('频率偏差', `${f.deviation_pct}%`, f.grade);
    }
    if (pqResult.harmonics) {
      add('电压THD', `${pqResult.harmonics.avg_thd_v}%`, pqResult.harmonics.grade);
      add('电流THD', `${pqResult.harmonics.avg_thd_i}%`, pqResult.harmonics.grade);
    }
    add('综合评级', pqResult.overall_grade || '', '');
    const csv = rows.join('\n');
    const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = `电能质量_${pqResult.device_name}_${new Date().toISOString().slice(0,10)}.csv`;
    a.click(); URL.revokeObjectURL(url);
  };

  const statusColor = (s: string) => s === '优' ? '#52c41a' : s === '良' ? '#faad14' : '#ff4d4f';
  const maxLoad = Math.max(...forecast.map(f => f.load_kw), 1);
  const periodColor = (n: string) => n === '谷时' ? '#52c41a' : n === '峰时' ? '#ff4d4f' : '#faad14';

  const effColumns = [
    { title: '设备', dataIndex: 'device_name', key: 'name' },
    { title: '类型', dataIndex: 'device_type', key: 'type', width: 100 },
    { title: '功率(kW)', dataIndex: 'power_kw', key: 'kw', width: 90, render: (v: number) => v.toFixed(1) },
    { title: '负荷率', dataIndex: 'load_pct', key: 'load', width: 80, render: (v: number) => `${v}%` },
    { title: 'COP', dataIndex: 'cop', key: 'cop', width: 70, render: (v: number) => v > 0 ? v.toFixed(1) : '-' },
    { title: '评分', dataIndex: 'efficiency', key: 'eff', width: 70,
      render: (v: number) => <span style={{ color: v >= 85 ? '#52c41a' : v >= 70 ? '#faad14' : '#ff4d4f', fontWeight: 600 }}>{v}</span> },
    { title: '状态', dataIndex: 'status', key: 'status', width: 70, render: (s: string) => <Tag color={statusColor(s)}>{s}</Tag> },
  ];

  // Helper: render bar chart
  const renderForecastChart = () => (
    <div style={{ display: 'flex', alignItems: 'flex-end', height: 160, gap: 2, padding: '0 20px' }}>
      {forecast.map((f, i) => {
        const h = (f.load_kw / maxLoad) * 120;
        const isPeak = f.load_kw === maxLoad;
        return (
          <div key={i} style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 0 }}>
            <span style={{ fontSize: 10, color: '#666', marginBottom: 2 }}>{f.load_kw.toFixed(0)}</span>
            <div style={{
              width: '100%', maxWidth: 28, height: h,
              background: isPeak ? 'linear-gradient(180deg, #ff4d4f, #ff7875)' : 'linear-gradient(180deg, #1890ff, #69c0ff)',
              borderRadius: '4px 4px 0 0',
            }} />
            <span style={{ fontSize: 10, color: '#999', marginTop: 4 }}>{f.hour}h</span>
          </div>
        );
      })}
    </div>
  );

  // Helper: render recommendation card
  const renderRecommendation = (r: Recommendation, i: number) => (
    <div key={i} style={{ padding: '10px 0', borderBottom: '1px solid #f0f0f0' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
        <strong style={{ fontSize: 13 }}>{r.device_name}</strong>
        <Tag color={r.priority === '高' ? 'red' : r.priority === '中' ? 'orange' : 'blue'} style={{ margin: 0, fontSize: 11 }}>{r.priority}优先级</Tag>
      </div>
      <div style={{ fontSize: 12, color: '#666', marginBottom: 4 }}>
        <span style={{ color: '#999' }}>{r.parameter}:</span>{' '}
        <span style={{ textDecoration: 'line-through', color: '#999' }}>{r.current_value}{r.unit}</span>
        {' → '}<span style={{ color: '#1890ff', fontWeight: 600 }}>{r.recommended_value}{r.unit}</span>
      </div>
      <div style={{ fontSize: 11, color: '#999' }}>{r.reason}</div>
      <div style={{ fontSize: 11, color: '#52c41a', marginTop: 2 }}>预估节电 {r.estimated_save_kw?.toFixed(1)} kW/h</div>
    </div>
  );

  // Tab: 智能分析
  const analysisTab = (
    <div>
      <Alert type="info" icon={<BulbOutlined />} title={<strong>AI 分析总结</strong>} description={summary} style={{ marginBottom: 16, borderRadius: 8 }} />
      <Row gutter={[12, 12]}>
        <Col xs={24} lg={14}>
          <Card title={<><ThunderboltOutlined /> 设备能效分析</>} size="small" style={{ marginBottom: 12 }}>
            <Table rowKey="device_id" columns={effColumns} dataSource={efficiencies} pagination={false} size="small" scroll={{ y: 300 }} />
          </Card>
          <Card title={<><LineChartOutlined /> 24h 负荷预测</>} size="small">
            {renderForecastChart()}
            <div style={{ fontSize: 12, color: '#999', textAlign: 'center', marginTop: 8 }}>
              预测室外温度 {forecast[0]?.temp || '--'}°C · 峰值 {maxLoad.toFixed(0)} kW @ {forecast.find(f => f.load_kw === maxLoad)?.hour || '--'}:00
            </div>
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card title={<><SettingOutlined /> 优化建议</>} size="small" styles={{ body: { padding: '8px 16px' } }}>
            {recommendations.map(renderRecommendation)}
          </Card>
        </Col>
      </Row>
    </div>
  );

  // Tab: 协同控制
  const strategyTab = (
    <div>
      <Alert type="success" icon={<BulbOutlined />} title={<strong>策略分析结果</strong>} description={stratSummary} style={{ marginBottom: 16, borderRadius: 8 }} />
      <Row gutter={[12, 12]}>
        {/* 电价时段 */}
        <Col xs={24}>
          <Card title={<><DollarOutlined /> 时段电价策略</>} size="small">
            {priceTactic && (
              <div>
                <div style={{ display: 'flex', gap: 8, marginBottom: 12, flexWrap: 'wrap' }}>
                  {priceTactic.periods.map((p, i) => (
                    <Tag key={i} color={periodColor(p.name)} style={{ fontSize: 13, padding: '4px 12px' }}>
                      {p.name} {p.start}:00-{p.end}:00 ¥{p.price}/kWh
                    </Tag>
                  ))}
                </div>
                <div style={{
                  background: '#fffbe6', border: '1px solid #ffe58f', borderRadius: 8,
                  padding: '10px 14px', fontSize: 13,
                }}>
                  <strong>当前 {priceTactic.current_hour}:00 · {priceTactic.current_period} · ¥{priceTactic.current_price}/kWh</strong>
                  <div style={{ color: '#666', marginTop: 4 }}>{priceTactic.recommendation}</div>
                </div>
              </div>
            )}
          </Card>
        </Col>

        {/* 主机-冷却塔联动 */}
        <Col xs={24} lg={12}>
          <Card title={<><ControlOutlined /> 主机-冷却塔联动</>} size="small" styles={{ body: { padding: '8px 16px' } }}>
            {linkages.map((l, i) => (
              <div key={i} style={{ padding: '10px 0', borderBottom: '1px solid #f0f0f0' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 6 }}>
                  <strong style={{ fontSize: 13 }}>{l.device_name}</strong>
                  {l.active ? <Tag color="blue">待执行</Tag> : <Tag color="green">已最优</Tag>}
                </div>
                <div style={{ fontSize: 12, color: '#666' }}>
                  冷却水温: <span style={{ textDecoration: 'line-through', color: '#999' }}>{l.current_cooling_temp}°C</span>
                  {' → '}<span style={{ color: '#1890ff', fontWeight: 600 }}>{l.target_cooling_temp}°C</span>
                  <span style={{ marginLeft: 12 }}>湿球温度: {l.wet_bulb_temp}°C</span>
                  <span style={{ marginLeft: 12 }}>COP 提升: +{l.cop_improvement}%</span>
                </div>
                <div style={{ fontSize: 11, color: '#999', marginTop: 4 }}>{l.reason}</div>
                {l.active && <div style={{ fontSize: 11, color: '#52c41a', marginTop: 2 }}>节电 {l.save_kw_per_hour.toFixed(1)} kW/h</div>}
              </div>
            ))}
          </Card>
        </Col>

        {/* 泵频率优化 */}
        <Col xs={24} lg={12}>
          <Card title={<><SyncOutlined /> 泵阀频率优化</>} size="small" styles={{ body: { padding: '8px 16px' } }}>
            {pumps.map((p, i) => (
              <div key={i} style={{ padding: '10px 0', borderBottom: '1px solid #f0f0f0' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 6 }}>
                  <strong style={{ fontSize: 13 }}>{p.device_name}</strong>
                  <Tag color="blue">{p.device_type}</Tag>
                  {p.active ? <Tag color="orange">待优化</Tag> : <Tag color="green">已最优</Tag>}
                </div>
                <div style={{ fontSize: 12, color: '#666' }}>
                  频率: <span style={{ textDecoration: 'line-through', color: '#999' }}>{p.current_freq}Hz</span>
                  {' → '}<span style={{ color: '#1890ff', fontWeight: 600 }}>{p.target_freq}Hz</span>
                  <span style={{ marginLeft: 12 }}>温差: {p.delta_t}°C</span>
                </div>
                <div style={{ fontSize: 11, color: '#999', marginTop: 4 }}>{p.reason}</div>
                {p.active && <div style={{ fontSize: 11, color: '#52c41a', marginTop: 2 }}>节电 {p.save_kw_per_hour.toFixed(1)} kW/h</div>}
              </div>
            ))}
          </Card>
        </Col>

        {/* 设备轮换 */}
        <Col xs={24}>
          <Card title={<><SyncOutlined spin /> 设备轮换计划</>} size="small">
            <Row gutter={[8, 8]}>
              {rotation.map((r, i) => (
                <Col xs={24} sm={12} md={6} key={i}>
                  <div style={{
                    border: '1px solid #f0f0f0', borderRadius: 8, padding: 10,
                    background: r.priority === 1 ? '#f6ffed' : r.priority === 2 ? '#fff7e6' : '#fafafa',
                  }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                      <strong style={{ fontSize: 12 }}>{r.device_name}</strong>
                      <Tag color={r.priority === 1 ? 'green' : r.priority === 2 ? 'orange' : 'default'} style={{ margin: 0, fontSize: 10 }}>
                        {r.recommendation}
                      </Tag>
                    </div>
                    <div style={{ fontSize: 11, color: '#999' }}>运行 {r.run_hours}h · {r.device_type}</div>
                    <div style={{ fontSize: 11, color: '#666', marginTop: 4 }}>{r.reason}</div>
                  </div>
                </Col>
              ))}
            </Row>
          </Card>
        </Col>
      </Row>
    </div>
  );

  // Tab: 电能质量
  const gradeColor = (g: string) => g === '优' ? '#52c41a' : g === '良' ? '#faad14' : g === '差' ? '#ff4d4f' : '#999';
  const pqTab = (
    <div>
      <Card size="small" style={{ marginBottom: 16 }}>
        <Space wrap>
          <Select
            placeholder="选择项目"
            style={{ minWidth: 160 }}
            value={selectedProject}
            allowClear
            onChange={(v) => { setSelectedProject(v); setSelectedBuilding(null); }}
            options={pqProjects.map((p: any) => ({ label: p.name, value: p.id }))}
          />
          <Select
            placeholder="选择楼宇"
            style={{ minWidth: 160 }}
            value={selectedBuilding}
            allowClear
            disabled={!selectedProject}
            onChange={(v) => setSelectedBuilding(v)}
            options={pqBuildings.filter((b: any) => Number(b.project_id) === Number(selectedProject)).map((b: any) => ({ label: b.name, value: b.id }))}
          />
          <Select
            placeholder="选择电表"
            style={{ minWidth: 180 }}
            value={selectedMeter}
            disabled={!selectedBuilding}
            onChange={(v) => { setSelectedMeter(v); setPqResult(null); }}
            options={meters.map(m => ({ label: m.name, value: m.id }))}
            notFoundContent={selectedBuilding ? '此楼宇暂无电表设备' : '请先选择楼宇'}
          />
          <RangePicker
            value={pqTimeRange as any}
            onChange={(dates) => setPqTimeRange(dates as [Dayjs, Dayjs] | null)}
            allowClear
            placeholder={['开始时间', '结束时间']}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={analyzePQ} loading={pqLoading}>分析</Button>
          {pqResult && <Button icon={<DownloadOutlined />} onClick={exportPQ}>导出CSV</Button>}
        </Space>
      </Card>
      {pqResult ? (
        <div>
          {/* Summary & Grade */}
          {pqResult.summary && (
            <Alert
              type={pqResult.overall_grade === '优' ? 'success' : pqResult.overall_grade === '差' ? 'error' : 'warning'}
              icon={<DashboardOutlined />}
              title={<strong>综合评级: <span style={{ color: gradeColor(pqResult.overall_grade), fontSize: 20 }}>{pqResult.overall_grade}</span> | {pqResult.device_name} ({pqResult.device_type})</strong>}
              description={pqResult.summary}
              style={{ marginBottom: 16, borderRadius: 8 }}
            />
          )}
          <Row gutter={[12, 12]}>
            {/* Voltage */}
            {pqResult.voltage && (
              <Col xs={24} md={12}>
                <Card size="small" title="电压质量">
                  <Descriptions column={2} size="small">
                    <Descriptions.Item label="平均电压">{pqResult.voltage.avg_v}V</Descriptions.Item>
                    <Descriptions.Item label="标称电压">{pqResult.voltage.nominal_v}V</Descriptions.Item>
                    <Descriptions.Item label="最小值">{pqResult.voltage.min_v}V</Descriptions.Item>
                    <Descriptions.Item label="最大值">{pqResult.voltage.max_v}V</Descriptions.Item>
                  </Descriptions>
                  <Divider style={{ margin: '8px 0' }} />
                  <Row gutter={8}>
                    <Col span={12}><Statistic title="电压偏差" value={pqResult.voltage.deviation_pct} suffix="%" precision={1} /></Col>
                    <Col span={12}><Statistic title="合格率" value={pqResult.voltage.qualified_rate} suffix="%" precision={1} /></Col>
                  </Row>
                  <Progress percent={pqResult.voltage.qualified_rate} strokeColor={pqResult.voltage.qualified_rate >= 95 ? '#52c41a' : '#faad14'} style={{ marginTop: 8 }} />
                  <Tag color={gradeColor(pqResult.voltage.grade)} style={{ marginTop: 8 }}>{pqResult.voltage.grade}</Tag>
                </Card>
              </Col>
            )}
            {/* Current Balance */}
            {pqResult.current && (
              <Col xs={24} md={12}>
                <Card size="small" title="三相电流平衡">
                  <Descriptions column={2} size="small">
                    <Descriptions.Item label="A相平均">{pqResult.current.avg_a}A</Descriptions.Item>
                    <Descriptions.Item label="B相平均">{pqResult.current.avg_b}A</Descriptions.Item>
                    <Descriptions.Item label="C相平均">{pqResult.current.avg_c}A</Descriptions.Item>
                    <Descriptions.Item label="最大相">{pqResult.current.max_phase_current}A</Descriptions.Item>
                  </Descriptions>
                  <Divider style={{ margin: '8px 0' }} />
                  <Statistic title="三相不平衡度" value={pqResult.current.imbalance_pct} suffix="%" precision={1} />
                  <Progress percent={Math.min(pqResult.current.imbalance_pct * 10, 100)} strokeColor={pqResult.current.imbalance_pct <= 15 ? '#52c41a' : '#ff4d4f'} style={{ marginTop: 8 }} />
                  <Tag color={gradeColor(pqResult.current.grade)} style={{ marginTop: 8 }}>{pqResult.current.grade}</Tag>
                </Card>
              </Col>
            )}
            {/* Power Factor */}
            {pqResult.power_factor && (
              <Col xs={24} md={12}>
                <Card size="small" title="功率因数">
                  <Descriptions column={2} size="small">
                    <Descriptions.Item label="平均功率因数">{pqResult.power_factor.avg_pf}</Descriptions.Item>
                    <Descriptions.Item label="最低值">{pqResult.power_factor.min_pf}</Descriptions.Item>
                  </Descriptions>
                  <Divider style={{ margin: '8px 0' }} />
                  <Statistic title="合格率" value={pqResult.power_factor.qualified_rate} suffix="%" precision={1} />
                  <Progress percent={pqResult.power_factor.qualified_rate} strokeColor={pqResult.power_factor.qualified_rate >= 90 ? '#52c41a' : '#faad14'} style={{ marginTop: 8 }} />
                  <Tag color={gradeColor(pqResult.power_factor.grade)} style={{ marginTop: 8 }}>{pqResult.power_factor.grade}</Tag>
                </Card>
              </Col>
            )}
            {/* Frequency */}
            {pqResult.frequency && (
              <Col xs={24} md={12}>
                <Card size="small" title="频率质量">
                  <Descriptions column={2} size="small">
                    <Descriptions.Item label="平均频率">{pqResult.frequency.avg_hz}Hz</Descriptions.Item>
                    <Descriptions.Item label="标称频率">{pqResult.frequency.nominal_hz}Hz</Descriptions.Item>
                    <Descriptions.Item label="最小值">{pqResult.frequency.min_hz}Hz</Descriptions.Item>
                    <Descriptions.Item label="最大值">{pqResult.frequency.max_hz}Hz</Descriptions.Item>
                  </Descriptions>
                  <Divider style={{ margin: '8px 0' }} />
                  <Statistic title="频率偏差" value={pqResult.frequency.deviation_pct} suffix="%" precision={2} />
                  <Tag color={gradeColor(pqResult.frequency.grade)} style={{ marginTop: 8 }}>{pqResult.frequency.grade}</Tag>
                </Card>
              </Col>
            )}
            {/* Harmonics */}
            {pqResult.harmonics && (
              <Col xs={24} md={12}>
                <Card size="small" title="谐波畸变 (THD)">
                  <Descriptions column={2} size="small">
                    <Descriptions.Item label="电压THD">{pqResult.harmonics.avg_thd_v}%</Descriptions.Item>
                    <Descriptions.Item label="电流THD">{pqResult.harmonics.avg_thd_i}%</Descriptions.Item>
                  </Descriptions>
                  <Divider style={{ margin: '8px 0' }} />
                  <Row gutter={8}>
                    <Col span={12}>
                      <Statistic title="电压THD" value={pqResult.harmonics.avg_thd_v} suffix="%" precision={1} />
                      <Progress percent={Math.min(pqResult.harmonics.avg_thd_v * 2, 100)} strokeColor={pqResult.harmonics.avg_thd_v <= 5 ? '#52c41a' : '#ff4d4f'} />
                    </Col>
                    <Col span={12}>
                      <Statistic title="电流THD" value={pqResult.harmonics.avg_thd_i} suffix="%" precision={1} />
                      <Progress percent={Math.min(pqResult.harmonics.avg_thd_i * 2, 100)} strokeColor={pqResult.harmonics.avg_thd_i <= 5 ? '#52c41a' : '#ff4d4f'} />
                    </Col>
                  </Row>
                  <Tag color={gradeColor(pqResult.harmonics.grade)} style={{ marginTop: 8 }}>{pqResult.harmonics.grade}</Tag>
                </Card>
              </Col>
            )}
          </Row>
        </div>
      ) : (
        <div style={{ textAlign: 'center', padding: 60, color: '#999' }}>
          <DashboardOutlined style={{ fontSize: 48, marginBottom: 16 }} />
          <div>选择电表和时间范围，点击「分析」查看电能质量报告</div>
        </div>
      )}
    </div>
  );

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}><RobotOutlined style={{ marginRight: 8, color: '#006875' }} />智能分析</h2>
      {loading || stratLoading ? (
        <div style={{ textAlign: 'center', padding: 80 }}><Spin size="large" description="加载中..." /></div>
      ) : (
        <Tabs defaultActiveKey="analysis" size="large" items={[
          { key: 'analysis', label: <span><ThunderboltOutlined /> 智能分析</span>, children: analysisTab },
          { key: 'strategies', label: <span><ControlOutlined /> 协同控制（预计节电 {totalSave}kW/h）</span>, children: strategyTab },
          { key: 'power-quality', label: <span><DashboardOutlined /> 电能质量</span>, children: pqTab },
        ]} />
      )}
    </div>
  );
}
