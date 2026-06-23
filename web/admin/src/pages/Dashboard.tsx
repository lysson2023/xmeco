import { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Modal, Input, message } from 'antd';
import { CheckCircleOutlined, WarningOutlined, ThunderboltOutlined, DashboardOutlined, ProjectOutlined, AimOutlined, EnvironmentOutlined, CloudOutlined, EditOutlined } from '@ant-design/icons';
import api from '../api/client';

interface WeatherNow {
  city_name: string;
  temp: string;
  feels_like: string;
  icon: string;
  weather_text: string;
  wind_dir: string;
  wind_scale: string;
  humidity: string;
  pressure: string;
  fetched_at: string;
}

export default function Dashboard() {
  const [cfg, setCfg] = useState<any>({});
  const [editKey, setEditKey] = useState('');
  const [editVal, setEditVal] = useState('');
  const [weather, setWeather] = useState<WeatherNow | null>(null);
  const [weatherLoading, setWeatherLoading] = useState(false);

  useEffect(() => {
    api.get('/dashboard').then(r => setCfg(r.data));
    // 获取第一个项目的天气（如果有关联城市的话）
    fetchFirstProjectWeather();
  }, []);

  const fetchFirstProjectWeather = async () => {
    setWeatherLoading(true);
    try {
      const { data: projects } = await api.get('/projects');
      if (projects.length > 0 && projects[0].city_id) {
        const { data: w } = await api.get(`/weather/project?project_id=${projects[0].id}`);
        setWeather(w);
      }
    } catch {
      // 静默处理——未配置城市或无 API key 时不显示
    } finally {
      setWeatherLoading(false);
    }
  };

  const save = async () => {
    await api.put('/dashboard', { [editKey]: editVal });
    setCfg({ ...cfg, [editKey]: editVal });
    setEditKey(''); setEditVal(''); message.success('已保存');
  };

  const cards = [
    { key: 'service_projects', title: '服务项目', icon: <ProjectOutlined style={{ color: '#1890ff' }} />, value: cfg.service_projects || '156', editable: true },
    { key: 'service_area', title: '服务面积', icon: <AimOutlined style={{ color: '#722ed1' }} />, value: cfg.service_area || '12.8万㎡', editable: true },
    { key: 'service_cities', title: '服务城市', icon: <EnvironmentOutlined style={{ color: '#13c2c2' }} />, value: cfg.service_cities || '8', editable: true },
    { key: 'power_saved', title: '节电总量', icon: <ThunderboltOutlined style={{ color: '#faad14' }} />, value: cfg.power_saved || '1,245', suffix: '度', editable: true },
    { key: 'carbon_saved', title: '节碳总量', icon: <CloudOutlined style={{ color: '#52c41a' }} />, value: cfg.carbon_saved || '986', suffix: '吨', editable: true },
    { key: 'online_devices', title: '在线设备', icon: <CheckCircleOutlined style={{ color: '#52c41a' }} />, value: cfg.online_devices || '2000' },
    { key: 'today_alarms', title: '今日告警', icon: <WarningOutlined style={{ color: '#ff4d4f' }} />, value: cfg.today_alarms || '2000' },
    { key: 'running_days', title: '运行天数', icon: <DashboardOutlined style={{ color: '#006875' }} />, value: cfg.running_days || '2000', suffix: '天' },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>系统概览</h2>

      {/* 天气卡片 */}
      {weather && (
        <Card
          size="small"
          style={{ marginBottom: 16, background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)', border: 'none' }}
        >
          <Row align="middle" justify="space-between">
            <Col>
              <div style={{ color: 'rgba(255,255,255,0.8)', fontSize: 13 }}>📍 {weather.city_name} 实时天气</div>
              <div style={{ color: '#fff', fontSize: 32, fontWeight: 700, marginTop: 4 }}>
                {weather.temp}°C
                <span style={{ fontSize: 16, fontWeight: 400, marginLeft: 8 }}>{weather.weather_text}</span>
              </div>
            </Col>
            <Col>
              <Row gutter={20}>
                <Col>
                  <div style={{ color: 'rgba(255,255,255,0.7)', fontSize: 12 }}>体感温度</div>
                  <div style={{ color: '#fff', fontSize: 16 }}>{weather.feels_like}°C</div>
                </Col>
                <Col>
                  <div style={{ color: 'rgba(255,255,255,0.7)', fontSize: 12 }}>湿度</div>
                  <div style={{ color: '#fff', fontSize: 16 }}>{weather.humidity}%</div>
                </Col>
                <Col>
                  <div style={{ color: 'rgba(255,255,255,0.7)', fontSize: 12 }}>风向</div>
                  <div style={{ color: '#fff', fontSize: 16 }}>{weather.wind_dir} {weather.wind_scale}级</div>
                </Col>
                <Col>
                  <div style={{ color: 'rgba(255,255,255,0.7)', fontSize: 12 }}>气压</div>
                  <div style={{ color: '#fff', fontSize: 16 }}>{weather.pressure} hPa</div>
                </Col>
              </Row>
            </Col>
          </Row>
        </Card>
      )}

      <Row gutter={[12, 12]}>
        {cards.map(c => (
          <Col xs={24} sm={12} lg={6} key={c.key}>
            <Card hoverable size="small">
              <Statistic
                title={<span>{c.title} {c.editable && <EditOutlined style={{ cursor: 'pointer', fontSize: 12, color: '#1890ff', marginLeft: 4 }} onClick={() => { setEditKey(c.key); setEditVal(c.value); }} />}</span>}
                value={c.value} suffix={c.suffix} prefix={c.icon} />
            </Card>
          </Col>
        ))}
      </Row>
      <Modal title="编辑" open={!!editKey} onOk={save} onCancel={() => setEditKey('')}>
        <Input value={editVal} onChange={e => setEditVal(e.target.value)} />
      </Modal>
    </div>
  );
}
