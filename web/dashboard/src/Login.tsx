import { useState } from 'react';
import axios from 'axios';
import './login.css';

interface Props { onLogin: () => void }

export default function Login({ onLogin }: Props) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault(); setLoading(true); setError('');
    try {
      const r = await axios.post('/api/v1/auth/login', { username, password });
      localStorage.setItem('token', r.data.token);
      onLogin();
    } catch (err: any) {
      setError(err?.response?.data?.error || '登录失败');
    } finally { setLoading(false); }
  };

  return (
    <div className="login-page">
      {/* Background grids */}
      <div className="bg-subgrid" /><div className="bg-maingrid" />
      {/* Fiber lines */}
      <div className="fiber" style={{ top: '15%' }} />
      <div className="fiber" style={{ top: '45%', animationDelay: '2s', width: '200px', opacity: 0.4 }} />
      <div className="fiber" style={{ top: '75%', animationDelay: '1.5s' }} />
      <div className="fiber" style={{ top: '90%', animationDelay: '3s', width: '300px' }} />
      {/* Junctions */}
      {[[20,30],[40,70],[60,20],[80,85],[15,55],[75,45]].map(([t,l],i) => (
        <div key={i} className="junction" style={{ top: t+'%', left: l+'%', animationDelay: i*0.4+'s' }} />
      ))}
      {/* Particles */}
      <div className="particle sm fast" style={{ top: '90%', left: '10%' }} />
      <div className="particle sm" style={{ top: '95%', left: '40%', animationDelay: '1s' }} />
      <div className="particle md" style={{ top: '85%', left: '70%', animationDelay: '2s' }} />
      <div className="particle lg" style={{ top: '10%', left: '30%' }} />
      <div className="particle lg" style={{ top: '80%', left: '60%', animationDelay: '5s' }} />
      {/* Central glow */}
      <div className="core-glow" />
      {/* Login card */}
      <div className="login-card">
        <div className="scan-line" />
        <div className="brand">
          <h1>熊猫智控 XMECO</h1>
          <p>多智能体能效节能系统</p>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="field">
            <label>用户名</label>
            <input type="text" placeholder="admin" value={username} onChange={e => setUsername(e.target.value)} />
          </div>
          <div className="field">
            <label>密码</label>
            <input type="password" placeholder="••••••••" value={password} onChange={e => setPassword(e.target.value)} />
          </div>
          {error && <div className="error-msg">{error}</div>}
          <button type="submit" disabled={loading}>
            {loading ? '登录中...' : '登录'}
            <span className="btn-shine" />
          </button>
        </form>
        <div className="footer-links">
          <span>深圳市高海拔科技有限公司</span>
        </div>
      </div>
      {/* Footer */}
      <div className="page-footer">
        <span>熊猫智控 XMECO 多智能体能效节能系统</span>
      </div>
    </div>
  );
}