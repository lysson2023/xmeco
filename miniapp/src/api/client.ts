// 小程序环境中 uni.request 要求绝对 HTTPS URL
// H5 模式使用相对路径即可（浏览器自动补全域名）
// 原生小程序构建时应替换为实际服务器地址
// #ifdef H5
const BASE = '/api/v1'
// #endif
// #ifndef H5
const BASE = 'https://api.example.com/api/v1' // TODO: 替换为实际 API 地址
// #endif
const REQUEST_TIMEOUT = 15000
let token = ''
let isRedirecting = false

type HttpMethod = 'OPTIONS' | 'GET' | 'HEAD' | 'POST' | 'PUT' | 'DELETE' | 'TRACE' | 'CONNECT'

// AuthError 表示因认证过期而触发的错误，页面 catch 中应跳过 Toast 提示
export class AuthError extends Error {
  constructor(msg: string) { super(msg); this.name = 'AuthError' }
}

function handle401() {
  token = ''
  uni.removeStorageSync('token')
  uni.removeStorageSync('user')
  if (!isRedirecting) {
    isRedirecting = true
    uni.reLaunch({
      url: '/pages/login/login',
      success: () => { isRedirecting = false },
      fail: () => { isRedirecting = false }
    })
    // 超时兜底，防止回调不触发导致永久死锁
    setTimeout(() => { isRedirecting = false }, 3000)
  }
}

async function request(url: string, method: HttpMethod, data?: any): Promise<any> {
  try {
    const r = await uni.request({
      url,
      method,
      data,
      timeout: REQUEST_TIMEOUT,
      header: { Authorization: 'Bearer ' + getToken() },
    })
    if (r.statusCode === 401) {
      handle401()
      throw new AuthError('登录已过期')
    }
    if (r.statusCode && r.statusCode >= 400) {
      const msg = (r.data as any)?.error || (r.data as any)?.message || '请求失败'
      throw new Error(msg)
    }
    return r
  } catch (e: any) {
    // 401 已由上方处理，直接抛出 AuthError
    if (e instanceof AuthError) throw e
    // Network errors or timeout
    if (e.errMsg && !e.message) {
      throw new Error('网络连接失败')
    }
    throw e
  }
}

function getToken(): string {
  if (!token) {
    token = uni.getStorageSync('token') || ''
  }
  return token
}

export function clearToken() {
  token = ''
  uni.removeStorageSync('token')
  uni.removeStorageSync('user')
}

export const api = {
  async login(u: string, p: string) {
    const r = await uni.request({
      url: BASE + '/auth/login',
      method: 'POST',
      data: { username: u, password: p },
      timeout: REQUEST_TIMEOUT,
    })
    if (r.statusCode === 401) {
      throw new Error('用户名或密码错误')
    }
    if (r.statusCode && r.statusCode >= 400) {
      throw new Error((r.data as any)?.error || '登录失败')
    }
    const d = r.data as any
    token = d.token
    uni.setStorageSync('token', token)
    if (d.user) {
      uni.setStorageSync('user', JSON.stringify(d.user))
    }
    return d
  },
  getToken,
  async get(path: string) { return request(BASE + path, 'GET') },
  async post(path: string, data: any) { return request(BASE + path, 'POST', data) },
  async put(path: string, data: any) { return request(BASE + path, 'PUT', data) },
  async del(path: string) { return request(BASE + path, 'DELETE') },
}
