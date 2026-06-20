package auth

import (
	"context"
	"errors"
)

var ErrPermissionDenied = errors.New("无此操作权限")

// HasPermission 检查用户是否有指定权限点。
// 返回 (有权限, error)。error != nil 表示数据库异常——调用方应返回 503 而非 403。
func (s *Service) HasPermission(ctx context.Context, userID int, permCode string) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM permission p
		 JOIN role_permission rp ON rp.perm_id = p.id
		 JOIN users u ON u.role_id = rp.role_id
		 WHERE u.id = $1 AND p.code = $2`, userID, permCode,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
