/**
 * 大屏专用 API 客户端 — Screen.tsx 和 DataCenter.tsx 共用
 * Token 存储在 localStorage 的 'screen_token' key
 * 401 时派发 'screen-auth-expired' 事件，由组件监听后退回登录页
 */
import axios from 'axios';
import { isTokenExpired } from '../utils/auth';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '/api/v1',
  timeout: 30000,
});

const setAuth = (t: string) => { api.defaults.headers.common['Authorization'] = 'Bearer ' + t; };

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
