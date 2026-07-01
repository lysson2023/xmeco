/**
 * 大屏专用 API 客户端 — Screen.tsx 和子组件共用
 * Token 存储在 localStorage 的 'screen_token' key
 * 401 时派发 'screen-auth-expired' 事件，由组件监听后退回登录页
 */
import axios from 'axios';
import { isTokenExpired } from '../utils/auth';

// AuthError 标记认证相关错误，页面 catch 中应跳过 Toast 提示
export class AuthError extends Error {
  constructor(msg: string) { super(msg); this.name = 'AuthError'; }
}

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '/api/v1',
  timeout: 30000,
});

// Set initial auth header
const setAuth = (t: string) => { api.defaults.headers.common['Authorization'] = 'Bearer ' + t; };

// Request interceptor: read fresh token from localStorage on every request
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('screen_token');
  if (token) {
    if (isTokenExpired(token)) {
      localStorage.removeItem('screen_token');
      localStorage.removeItem('screen_user');
      window.dispatchEvent(new CustomEvent('screen-auth-expired'));
      return Promise.reject(new AuthError('Token expired'));
    }
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Auto-logout on 401
api.interceptors.response.use(
  (r) => r,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('screen_token');
      localStorage.removeItem('screen_user');
      window.dispatchEvent(new CustomEvent('screen-auth-expired'));
    }
    return Promise.reject(err);
  }
);

export { api, setAuth, isTokenExpired };
