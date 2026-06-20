import { useState } from 'react';
import { Form, Input, Button, Card, message } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '../api/client';

export default function LoginPage() {
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true);
    try {
      const res = await api.post('/auth/login', values);
      localStorage.setItem('token', res.data.token);
      localStorage.setItem('user', JSON.stringify(res.data.user));
      message.success('登录成功');
      navigate('/');
    } catch (err: any) {
      const msg = err?.response?.data?.error || '登录失败';
      message.error(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'linear-gradient(135deg, #0d1c32, #1a3a5c)' }}>
      <Card style={{ width: 400, borderRadius: 12 }} title={<div style={{ textAlign: 'center', fontSize: 22, fontWeight: 700, color: '#006875' }}>XMECO</div>}>
        <Form onFinish={onFinish} size="large">
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}><Input prefix={<UserOutlined />} placeholder="用户名" /></Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}><Input.Password prefix={<LockOutlined />} placeholder="密码" /></Form.Item>
          <Form.Item><Button type="primary" htmlType="submit" loading={loading} block>登录</Button></Form.Item>
        </Form>
      </Card>
    </div>
  );
}
