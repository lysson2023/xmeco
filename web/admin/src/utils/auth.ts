/**
 * JWT token 工具函数 — 管理后台和大屏共用
 */

/**
 * 检查 JWT token 是否已过期。
 * 解析 payload 的 exp 字段（Unix 秒），与当前时间比较。
 * 任何解析错误或缺失 exp 都视为已过期（返回 true），确保安全。
 */
export function isTokenExpired(token: string): boolean {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return true;
    // Normalize base64url to base64 (JWT uses base64url encoding)
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const payload = JSON.parse(atob(base64));
    const exp = Number(payload.exp);
    if (!Number.isFinite(exp) || exp <= 0) return true;
    return exp * 1000 < Date.now();
  } catch {
    return true;
  }
}
