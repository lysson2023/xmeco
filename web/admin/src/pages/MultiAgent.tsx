import { useEffect, useState } from 'react';
import { Card, Row, Col, Table, Tag, Spin, Alert, Tabs, message } from 'antd';
import {
  ThunderboltOutlined, LineChartOutlined, SettingOutlined,
  RobotOutlined, BulbOutlined, ControlOutlined, DollarOutlined, SyncOutlined,
} from '@ant-design/icons';
import api from '../api/client';

// ---- Types ----
interface EfficiencyItem {
  device_id: number; device_name: string; device_type: string;
  power_kw: number; load_pct: number; cop: number; efficiency: number; status: string;
}
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
      <Alert type="info" icon={<BulbOutlined />} message={<strong>AI 分析总结</strong>} description={summary} style={{ marginBottom: 16, borderRadius: 8 }} />
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
          <Card title={<><SettingOutlined /> 优化建议</>} size="small" bodyStyle={{ padding: '8px 16px' }}>
            {recommendations.map(renderRecommendation)}
          </Card>
        </Col>
      </Row>
    </div>
  );

  // Tab: 协同控制
  const strategyTab = (
    <div>
      <Alert type="success" icon={<BulbOutlined />} message={<strong>策略分析结果</strong>} description={stratSummary} style={{ marginBottom: 16, borderRadius: 8 }} />
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
          <Card title={<><ControlOutlined /> 主机-冷却塔联动</>} size="small" bodyStyle={{ padding: '8px 16px' }}>
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
          <Card title={<><SyncOutlined /> 泵阀频率优化</>} size="small" bodyStyle={{ padding: '8px 16px' }}>
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

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}><RobotOutlined style={{ marginRight: 8, color: '#006875' }} />智能分析</h2>
      {loading || stratLoading ? (
        <div style={{ textAlign: 'center', padding: 80 }}><Spin size="large" tip="加载中..." /></div>
      ) : (
        <Tabs defaultActiveKey="analysis" size="large" items={[
          { key: 'analysis', label: <span><ThunderboltOutlined /> 智能分析</span>, children: analysisTab },
          { key: 'strategies', label: <span><ControlOutlined /> 协同控制（预计节电 {totalSave}kW/h）</span>, children: strategyTab },
        ]} />
      )}
    </div>
  );
}
