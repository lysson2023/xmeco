import axios from 'axios';
import { isTokenExpired } from '../utils/auth';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '/api/v1',
  timeout: 30000,
});

// Prevent duplicate auth-expired events when multiple requests
// fail concurrently (e.g. token expires mid-session).
let authExpiredFired = false;
function fireAuthExpired() {
  if (authExpiredFired) return;
  authExpiredFired = true;
  localStorage.removeItem('token');
  localStorage.removeItem('user');
  window.dispatchEvent(new CustomEvent('auth-expired'));
}

// resetAuthExpiredFlag resets the dedup guard so that after a fresh login,
// a future token expiry will trigger the auth-expired flow again.
export function resetAuthExpiredFlag() {
  authExpiredFired = false;
}

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    if (isTokenExpired(token)) {
      fireAuthExpired();
      return Promise.reject(new Error('Token expired'));
    }
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      fireAuthExpired();
    }
    return Promise.reject(err);
  }
);

export default api;
