import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { message } from 'antd';
import api from '../api/client';
import './Login.css';

export default function LoginPage() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username || !password) {
      message.warning('请输入用户名和密码');
      return;
    }
    setLoading(true);
    try {
      const res = await api.post('/auth/login', { username, password });
      localStorage.setItem('token', res.data.token);
      localStorage.setItem('user', JSON.stringify(res.data.user));
      message.success('登录成功');
      navigate('/', { replace: true });
    } catch (err: any) {
      const msg = err?.response?.data?.error || '登录失败';
      message.error(msg || '登录失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-wrapper">
      {/* Background Layers */}
      <div className="bg-layer">
        <div className="bg-subgrid" />
        <div className="bg-grid" />
        {/* Fiber Lines */}
        <div className="fiber-line" style={{ top: '15%', animationDelay: '0s' }} />
        <div className="fiber-line" style={{ top: '45%', animationDelay: '2s', width: '200px', opacity: 0.4 }} />
        <div className="fiber-line" style={{ top: '75%', animationDelay: '1.5s' }} />
        <div className="fiber-line" style={{ top: '90%', animationDelay: '3s', width: '300px' }} />
        {/* Junction Points */}
        <div className="junction" style={{ top: '20%', left: '30%' }} />
        <div className="junction" style={{ top: '40%', left: '70%', animationDelay: '0.5s' }} />
        <div className="junction" style={{ top: '60%', left: '20%', animationDelay: '1.2s' }} />
        <div className="junction" style={{ top: '80%', left: '85%', animationDelay: '0.8s' }} />
        <div className="junction" style={{ top: '15%', left: '55%', animationDelay: '1.5s' }} />
        <div className="junction" style={{ top: '75%', left: '45%', animationDelay: '2s' }} />
        <div className="bg-glow" />
      </div>

      {/* Particles */}
      <div className="particles-layer">
        <div className="particle sm" style={{ top: '90%', left: '10%', '--tx': '100px', '--ty': '-900px' } as React.CSSProperties} />
        <div className="particle sm" style={{ top: '95%', left: '40%', '--tx': '-50px', '--ty': '-900px', animationDelay: '1s' } as React.CSSProperties} />
        <div className="particle" style={{ top: '85%', left: '70%', '--tx': '200px', '--ty': '-800px', animationDelay: '2s' } as React.CSSProperties} />
        <div className="particle md" style={{ top: '50%', left: '50%', '--tx': '300px', '--ty': '-200px', animationDelay: '0.5s' } as React.CSSProperties} />
        <div className="particle md" style={{ top: '20%', left: '80%', '--tx': '-400px', '--ty': '400px', animationDelay: '3s', opacity: 0.5 } as React.CSSProperties} />
        <div className="particle md" style={{ top: '60%', left: '20%', '--tx': '500px', '--ty': '100px' } as React.CSSProperties} />
        <div className="particle lg" style={{ top: '10%', left: '30%', '--tx': '100px', '--ty': '800px' } as React.CSSProperties} />
        <div className="particle lg" style={{ top: '80%', left: '60%', '--tx': '-200px', '--ty': '-700px', animationDelay: '5s' } as React.CSSProperties} />
      </div>

      {/* Central Hub */}
      <div className="central-hub">
        <div className="hub-core" />
        <div className="hub-ring ring-1" />
        <div className="hub-ring ring-2" />
      </div>

      {/* Login Card */}
      <main className="login-card-wrapper">
        <div className="login-card">
          <div className="scan-line" />
          
          {/* Brand */}
          <div className="brand-area">
            <h1 className="brand-title">熊猫智控<span className="brand-sub">XMECO</span></h1>
            <p className="brand-tagline">AIOT 节能管理平台</p>
          </div>

          {/* Form */}
          <form className="login-form" onSubmit={handleLogin}>
            <div className="input-group">
              <label className="input-label">用户名</label>
              <div className="input-wrap">
                <input
                  className="login-input"
                  type="text"
                  placeholder="请输入用户名"
                  value={username}
                  onChange={e => setUsername(e.target.value)}
                />
                <div className="input-underline" />
                <span className="input-icon">👤</span>
              </div>
            </div>

            <div className="input-group">
              <label className="input-label">密码</label>
              <div className="input-wrap">
                <input
                  className="login-input"
                  type="password"
                  placeholder="••••••••••"
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                />
                <div className="input-underline" />
                <span className="input-icon">🔒</span>
              </div>
            </div>

            <button className="login-btn" type="submit" disabled={loading}>
              <span className="btn-text">{loading ? '登录中…' : '登录'}</span>
              
              <div className="btn-shine" />
            </button>

            
          </form>
        </div>
      </main>

      {/* Footer */}
      <footer className="login-footer"><div>© 深圳市高海拔科技有限公司</div></footer>
    </div>
  );
}
