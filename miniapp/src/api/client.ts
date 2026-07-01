// 小程序环境中 uni.request 要求绝对 HTTPS URL
// H5 模式使用相对路径即可（浏览器自动补全域名）
// #ifdef H5
const BASE = '/api/v1'
// #endif
// #ifndef H5
// 生产环境API地址 - 请根据实际部署情况修改
const BASE = 'https://xmeco.highaltitude.cn/api/v1'
// #endif
const REQUEST_TIMEOUT = 15000
let token = ''
let isRedirecting = false

type HttpMethod = 'OPTIONS' | 'GET' | 'HEAD' | 'POST' | 'PUT' | 'DELETE' | 'TRACE' | 'CONNECT'

// AuthError 表示因认证过期而触发的错误，页面 catch 中应跳过 Toast 提示
export class AuthError extends Error {
  constructor(msg: string) {
    super(msg)
    this.name = 'AuthError'
    // 修复 ES5 转译下 instanceof 检查失败的问题
    Object.setPrototypeOf(this, AuthError.prototype)
  }
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

async function request(url: string, method: HttpMethod, data?: any, skipAuth?: boolean): Promise<any> {
  try {
    const header: Record<string, string> = {}
    if (!skipAuth) {
      header.Authorization = 'Bearer ' + getToken()
    }
    const r = await uni.request({
      url,
      method,
      data,
      timeout: REQUEST_TIMEOUT,
      header,
    })
    if (r.statusCode === 401) {
      if (!skipAuth) {
        handle401()
      }
      throw new AuthError(skipAuth ? '用户名或密码错误' : '登录已过期')
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
    // request() 统一处理 401/4xx/5xx，skipAuth=true 表示登录页不需要全局登出跳转
    const r = await request(BASE + '/auth/login', 'POST', { username: u, password: p }, true)
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
