package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserInactive       = errors.New("用户已被禁用")
)

type Service struct {
	pool      *pgxpool.Pool
	jwtSecret []byte
}

type User struct {
	ID               int      `json:"id"`
	Username         string   `json:"username"`
	RoleID           int      `json:"role_id"`
	RoleCode         string   `json:"role_code"`
	RoleLevel        int      `json:"role_level"`
	AgentID          *int     `json:"agent_id"`
	DefaultProjectID *int     `json:"default_project_id"`
	Permissions      []string `json:"permissions"`
}

type Claims struct {
	UserID   int    `json:"uid"`
	Username string `json:"uname"`
	RoleCode string `json:"role"`
	jwt.RegisteredClaims
}

func New(pool *pgxpool.Pool, secret string) *Service {
	return &Service{pool: pool, jwtSecret: []byte(secret)}
}

// Login 验证用户名密码，返回 JWT token
func (s *Service) Login(ctx context.Context, username, password string) (string, *User, error) {
	var user User
	var passwordHash string
	var isActive bool
	err := s.pool.QueryRow(ctx,
		`SELECT u.id, u.username, u.password_hash, u.role_id, r.code, r.level, u.agent_id, u.default_project_id, u.is_active
		 FROM users u JOIN role r ON r.id = u.role_id
		 WHERE u.username = $1`, username,
	).Scan(&user.ID, &user.Username, &passwordHash, &user.RoleID, &user.RoleCode, &user.RoleLevel, &user.AgentID, &user.DefaultProjectID, &isActive)
	if err != nil {
		return "", nil, ErrInvalidCredentials
	}
	if !isActive {
		return "", nil, ErrUserInactive
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	// 加载权限
	rows, err := s.pool.Query(ctx,
		`SELECT p.code FROM permission p
		 JOIN role_permission rp ON rp.perm_id = p.id
		 WHERE rp.role_id = $1`, user.RoleID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return "", nil, err
		}
		user.Permissions = append(user.Permissions, code)
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}

	// 更新最后登录时间
	s.pool.Exec(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, user.ID)

	// 生成 JWT
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		RoleCode: user.RoleCode,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "xmeco",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", nil, err
	}

	return tokenStr, &user, nil
}

// ValidateToken 验证 JWT，返回 Claims
func (s *Service) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// HashPassword 生成 bcrypt 哈希（用于创建用户）
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 验证密码与 bcrypt 哈希是否匹配
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
