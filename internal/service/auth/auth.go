package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"xmeco/internal/repository/postgres"
)

// tokenVersionCacheTTL is the TTL for the in-memory token_version cache.
const tokenVersionCacheTTL = 30 * time.Second

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrInternal           = errors.New("服务内部错误")
)

// secretBytes prevents accidental jwtSecret leakage via fmt %v/%+v or slog.
type secretBytes []byte

func (s secretBytes) String() string { return "***" }

type Service struct {
	pool              postgres.DBTX
	jwtSecret         secretBytes
	tokenVersionCache sync.Map
	stop              chan struct{}
	skipVersionCheck  bool // true in tests to skip DB-dependent token version validation
}

type tokenVersionEntry struct {
	version  int
	expireAt time.Time
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
	UserID       int    `json:"uid"`
	Username     string `json:"uname"`
	RoleCode     string `json:"role"`
	RoleLevel    *int   `json:"rlv,omitempty"`
	TokenVersion int    `json:"tver"`
	jwt.RegisteredClaims
}

func New(pool postgres.DBTX, secret string) *Service {
	svc := &Service{pool: pool, jwtSecret: secretBytes(secret), stop: make(chan struct{})}
	go svc.cleanupTokenVersionCache()
	return svc
}

// cleanupTokenVersionCache periodically deletes expired entries.
func (s *Service) cleanupTokenVersionCache() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.tokenVersionCache.Range(func(key, value any) bool {
				entry := value.(tokenVersionEntry)
				if time.Now().After(entry.expireAt) {
					s.tokenVersionCache.Delete(key)
				}
				return true
			})
		}
	}
}

// Shutdown gracefully stops the background cache cleanup goroutine.
// Safe to call multiple times.
func (s *Service) Shutdown() {
	if s == nil {
		return
	}
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}

// Pool exposes the database connection pool for auxiliary queries.
func (s *Service) Pool() postgres.DBTX { return s.pool }

// Login 验证用户名密码，返回 JWT token
func (s *Service) Login(ctx context.Context, username, password string) (string, *User, error) {
	var user User
	var passwordHash string
	var isActive bool
	var tokenVersion int
	var permsStr string
	err := s.pool.QueryRow(ctx,
		`SELECT u.id, u.username, u.password_hash, u.role_id, r.code, r.level, u.agent_id, u.default_project_id, u.is_active, u.token_version,
		       COALESCE((SELECT string_agg(p.code, ',') FROM permission p JOIN role_permission rp ON rp.perm_id=p.id WHERE rp.role_id=u.role_id), '')
		 FROM users u JOIN role r ON r.id = u.role_id
		 WHERE u.username = $1`, username,
	).Scan(&user.ID, &user.Username, &passwordHash, &user.RoleID, &user.RoleCode, &user.RoleLevel, &user.AgentID, &user.DefaultProjectID, &isActive, &tokenVersion, &permsStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, ErrInvalidCredentials
		}
		slog.Error("Login DB query failed", "username", username, "err", err)
		return "", nil, fmt.Errorf("%w: %v", ErrInternal, err)
	}
	// Verify password FIRST to prevent user enumeration via inactive check
	if err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}
	if !isActive {
		return "", nil, ErrInvalidCredentials
	}

	if permsStr != "" {
		user.Permissions = strings.Split(permsStr, ",")
	}

	// 更新最后登录时间 (best-effort, log on failure)
	if _, e := s.pool.Exec(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, user.ID); e != nil {
		slog.Warn("update last_login_at failed", "user", user.ID, "err", e)
	}

	// 生成 JWT
	claims := Claims{
		UserID:       user.ID,
		Username:     user.Username,
		RoleCode:     user.RoleCode,
		RoleLevel:    &user.RoleLevel,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "xmeco",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// must convert secretBytes → []byte because jwt.HMAC Sign does a strict type assertion
	tokenStr, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", nil, err
	}

	return tokenStr, &user, nil
}

// ValidateToken 验证 JWT，返回 Claims
func (s *Service) ValidateToken(ctx context.Context, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if s.pool != nil && !s.skipVersionCheck {
		dbVersion, err := s.getCachedTokenVersion(ctx, claims.UserID)
		if err != nil {
			// Fail-secure: if we cannot verify token version, reject the token.
			// Allowing a possibly-revoked token is worse than a transient auth failure.
			slog.Error("token version check failed - rejecting token", "user", claims.UserID, "err", err)
			return nil, fmt.Errorf("token validation unavailable")
		}
		if claims.TokenVersion != dbVersion {
			return nil, errors.New("token has been revoked - please re-login")
		}
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

func (s *Service) getCachedTokenVersion(ctx context.Context, userID int) (int, error) {
	if cached, ok := s.tokenVersionCache.Load(userID); ok {
		entry := cached.(tokenVersionEntry)
		if time.Now().Before(entry.expireAt) {
			return entry.version, nil
		}
	}
	var dbVersion int
	if err := s.pool.QueryRow(ctx, `SELECT token_version FROM users WHERE id=$1`, userID).Scan(&dbVersion); err != nil {
		return 0, err
	}
	s.tokenVersionCache.Store(userID, tokenVersionEntry{
		version:  dbVersion,
		expireAt: time.Now().Add(tokenVersionCacheTTL),
	})
	return dbVersion, nil
}

func (s *Service) InvalidateTokenVersion(userID int) {
	if s == nil {
		return
	}
	s.tokenVersionCache.Delete(userID)
}
