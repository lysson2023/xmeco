const BASE = 'http://localhost:9090/api/v1'
let token = ''
export const api = {
  async login(u: string, p: string) {
    const r = await uni.request({ url: BASE+'/auth/login', method: 'POST', data: { username: u, password: p } })
    token = (r.data as any).token; uni.setStorageSync('token', token); return r.data
  },
  getToken() { if (!token) token = uni.getStorageSync('token') || ''; return token },
  async get(path: string) { return uni.request({ url: BASE+path, header: { Authorization: 'Bearer '+this.getToken() } }) },
  async post(path: string, data: any) { return uni.request({ url: BASE+path, method: 'POST', data, header: { Authorization: 'Bearer '+this.getToken() } }) }
}
