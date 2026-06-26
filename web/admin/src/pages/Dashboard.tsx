import { useEffect, useMemo, useState } from 'react';
import { Card, Col, Row, Statistic, Spin, Select, Table, Tag, Typography, message } from 'antd';
import {
  ProjectOutlined, HomeOutlined, ApiOutlined, AlertOutlined,
  ThunderboltOutlined, EnvironmentOutlined, ClockCircleOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '../api/client';

const { Title } = Typography;

export default function Dashboard() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [projects, setProjects] = useState<any[]>([]);
  const [pid, setPid] = useState<number | null>(null);
  const [data, setData] = useState<any>({});

  // Load project list
  useEffect(() => {
    api.get('/projects').then(r => {
      setProjects(r.data || []);
    }).catch(() => { message.error('项目列表加载失败'); });
  }, []);

  // Fetch screen data when project selected
  useEffect(() => {
    if (!pid) return;
    setLoading(true);
    api.get('/screen/data', { params: { project_id: pid } })
      .then(r => setData(r.data))
      .catch(() => { message.error('大屏数据加载失败'); })
      .finally(() => setLoading(false));
  }, [pid]);

  // Fetch recent alarms
  const [alarms, setAlarms] = useState<any[]>([]);
  useEffect(() => {
    api.get('/alarm-logs', { params: { today: '1' } })
      .then(r => {
        const list = r.data || [];
        setAlarms(Array.isArray(list) ? list : []);
      })
      .catch(() => { message.error('告警日志加载失败'); });
  }, []);

  // Fetch system stats (parallel, merged atomically to avoid race)
  const [stats, setStats] = useState<any>({});
  useEffect(() => {
    Promise.all([
      api.get('/system/db-stats'),
      api.get('/system/info'),
    ]).then(([dbRes, infoRes]) => {
      setStats({ ...dbRes.data, ...infoRes.data });
    }).catch(() => { message.error('系统信息加载失败'); });
  }, []);

  const buildingsCount = data.buildings?.length || 0;
  const devicesCount = data.devices?.length || 0;
  const savingRate = data.saving_rate ? (data.saving_rate * 100).toFixed(1) : '--';
  const powerSaved = data.power_saved?.toFixed(1) || '0.0';

  const alarmCols = useMemo(() => [
    { title: '时间', dataIndex: 'created_at', key: 'time', width: 160, render: (v: string) => v?.slice(0, 16) },
    { title: '设备', dataIndex: 'device_name', key: 'device', width: 120 },
    { title: '信息', dataIndex: 'message', key: 'msg', ellipsis: true },
    { title: '级别', dataIndex: 'level', key: 'level', width: 80,
      render: (v: string) => {
        if (v === 'critical') return <Tag color="red">严重</Tag>;
        if (v === 'warning') return <Tag color="orange">警告</Tag>;
        return <Tag color="blue">信息</Tag>;
      }
    },
    { title: '状态', dataIndex: 'ack_at', key: 'acked', width: 80,
      render: (v: string | null) => v ? <Tag color="green">已确认</Tag> : <Tag color="red">未确认</Tag>
    },
  ], []);

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <Title level={4} style={{ margin: 0 }}>仪表盘</Title>
        <Select
          placeholder="选择项目查看概览"
          allowClear
          showSearch
          style={{ width: 280 }}
          value={pid}
          onChange={(v) => setPid(v || null)}
          options={projects.map((p: any) => ({ label: p.name, value: p.id }))}
          filterOption={(input, option) => (option?.label as string)?.includes(input)}
        />
      </div>

      {/* Summary Cards */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={12} sm={8} md={6}>
          <Card hoverable onClick={() => navigate('/projects')}>
            <Statistic title="项目数" value={projects.length} prefix={<ProjectOutlined />} valueStyle={{ color: '#006875' }} />
          </Card>
        </Col>
        <Col xs={12} sm={8} md={6}>
          <Card hoverable onClick={() => navigate('/buildings')}>
            <Statistic title="楼宇数" value={buildingsCount} prefix={<HomeOutlined />} valueStyle={{ color: '#1677ff' }} />
          </Card>
        </Col>
        <Col xs={12} sm={8} md={6}>
          <Card hoverable onClick={() => navigate('/devices')}>
            <Statistic title="设备数" value={devicesCount} prefix={<ApiOutlined />} valueStyle={{ color: '#52c41a' }} />
          </Card>
        </Col>
        <Col xs={12} sm={8} md={6}>
          <Card hoverable onClick={() => navigate('/alarms')}>
            <Statistic
              title="未确认告警"
              value={alarms.filter((a: any) => !a.acked).length}
              prefix={<AlertOutlined />}
              valueStyle={{ color: '#ff4d4f' }}
            />
          </Card>
        </Col>
      </Row>

      {/* Energy & Weather Row */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} md={12}>
          <Card title={<><ThunderboltOutlined /> 能效概览</>}>
            <Spin spinning={loading}>
              {pid ? (
                <Row gutter={16}>
                  <Col span={8}>
                    <Statistic title="节能率" value={savingRate} suffix="%" valueStyle={{ color: '#52c41a' }} />
                  </Col>
                  <Col span={8}>
                    <Statistic title="节电量" value={powerSaved} suffix="kWh" valueStyle={{ color: '#006875' }} />
                  </Col>
                  <Col span={8}>
                    <Statistic title="运行天数" value={data.running_days || 0} suffix="天" valueStyle={{ color: '#fa8c16' }} />
                  </Col>
                  <Col span={8} style={{ marginTop: 16 }}>
                    <Statistic title="节碳量" value={data.carbon_saved?.toFixed(1) || '0.0'} suffix="kg" valueStyle={{ color: '#13c2c2' }} />
                  </Col>
                  <Col span={8} style={{ marginTop: 16 }}>
                    <Statistic title="总功率" value={data.meter_power?.toFixed(1) || '0.0'} suffix="kW" valueStyle={{ color: '#ff4d4f' }} />
                  </Col>
                  <Col span={8} style={{ marginTop: 16 }}>
                    <Statistic title="电表数" value={data.meters?.length || 0} suffix="个" />
                  </Col>
                </Row>
              ) : (
                <div style={{ color: '#999', textAlign: 'center', padding: 32 }}>请选择项目查看能效数据</div>
              )}
            </Spin>
          </Card>
        </Col>
        <Col xs={24} md={12}>
          <Card title={<><EnvironmentOutlined /> 今日天气</>}>
            {data.weather ? (
              <Row gutter={16}>
                <Col span={12}>
                  <Statistic title="城市" value={data.weather.city} prefix={<EnvironmentOutlined />} />
                </Col>
                <Col span={12}>
                  <Statistic title="温度" value={data.weather.temp} suffix="°C" valueStyle={{ color: '#fa8c16' }} />
                </Col>
                <Col span={12} style={{ marginTop: 12 }}>
                  <div>天气: {data.weather.text}</div>
                </Col>
                <Col span={12} style={{ marginTop: 12 }}>
                  <div>湿度: {data.weather.humidity}% | {data.weather.wind_dir} {data.weather.wind_scale}级</div>
                </Col>
              </Row>
            ) : (
              <div style={{ color: '#999', textAlign: 'center', padding: 32 }}>
                {pid ? '暂无天气数据' : '请选择项目查看天气'}
              </div>
            )}
          </Card>
        </Col>
      </Row>

      {/* Recent Alarms */}
      <Card title={<><AlertOutlined /> 最近告警</>} style={{ marginBottom: 24 }}>
        <Table
          dataSource={alarms.slice(0, 8)}
          columns={alarmCols}
          rowKey="id"
          size="small"
          pagination={false}
          onRow={(record) => ({
            onClick: () => navigate('/alarms'),
            style: { cursor: 'pointer' },
          })}
        />
      </Card>

      {/* System Info */}
      <Row gutter={[16, 16]}>
        <Col xs={24} md={12}>
          <Card title={<><ClockCircleOutlined /> 最近定时任务</>}>
            {data.tasks?.length ? (
              (data.tasks || []).slice(0, 5).map((t: any, i: number) => (
                <div key={i} style={{ padding: '6px 0', borderBottom: '1px solid #f0f0f0', display: 'flex', justifyContent: 'space-between' }}>
                  <span style={{ fontWeight: 500 }}>{t.time}</span>
                  <span>{t.device}</span>
                  <Tag color={t.enabled ? 'green' : 'default'}>{t.enabled ? '启用' : '停用'}</Tag>
                </div>
              ))
            ) : (
              <div style={{ color: '#999', textAlign: 'center', padding: 24 }}>暂无定时任务</div>
            )}
          </Card>
        </Col>
        <Col xs={24} md={12}>
          <Card title="系统信息">
            <Row gutter={16}>
              <Col span={12}><Statistic title="数据库表" value={stats.tables || '--'} /></Col>
              <Col span={12}><Statistic title="迁移版本" value={stats.migrations || '--'} /></Col>
              <Col span={12} style={{ marginTop: 12 }}><Statistic title="服务状态" value={stats.status || '运行中'} valueStyle={{ color: '#52c41a' }} /></Col>
              <Col span={12} style={{ marginTop: 12 }}><Statistic title="服务时间" value={stats.time?.slice(0, 19) || '--'} /></Col>
            </Row>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
