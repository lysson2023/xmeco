import { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Modal, Input, message } from 'antd';
import { CheckCircleOutlined, WarningOutlined, ThunderboltOutlined, DashboardOutlined, ProjectOutlined, AimOutlined, EnvironmentOutlined, CloudOutlined, EditOutlined } from '@ant-design/icons';
import api from '../api/client';

export default function Dashboard() {
  const [cfg, setCfg] = useState<any>({});
  const [editKey, setEditKey] = useState('');
  const [editVal, setEditVal] = useState('');

  useEffect(() => { api.get('/dashboard').then(r => setCfg(r.data)); }, []);

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