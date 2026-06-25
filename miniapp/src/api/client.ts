const BASE = '/api/v1'
let token = ''

type HttpMethod = 'OPTIONS' | 'GET' | 'HEAD' | 'POST' | 'PUT' | 'DELETE' | 'TRACE' | 'CONNECT'

async function request(url: string, method: HttpMethod, data?: any): Promise<any> {
  const r = await uni.request({ url, method, data, header: { Authorization: 'Bearer ' + getToken() } })
  if (r.statusCode === 401) {
    uni.removeStorageSync('token')
    uni.reLaunch({ url: '/pages/login/login' })
    throw new Error('token expired')
  }
  return r
}

function getToken(): string {
  if (!token) token = uni.getStorageSync('token') || ''
  return token
}

export const api = {
  async login(u: string, p: string) {
    const r = await uni.request({ url: BASE + '/auth/login', method: 'POST', data: { username: u, password: p } })
    token = (r.data as any).token
    uni.setStorageSync('token', token)
    return r.data
  },
  getToken,
  async get(path: string) { return request(BASE + path, 'GET') },
  async post(path: string, data: any) { return request(BASE + path, 'POST', data) },
  async put(path: string, data: any) { return request(BASE + path, 'PUT', data) },
  async del(path: string) { return request(BASE + path, 'DELETE') },
}
