package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DBHost                  string
	DBPort                  string
	DBUser                  string
	DBPassword              string
	DBName                  string
	DBSSLMode               string
	ServerPort              string
	JWTSecret               string
	AllowedOrigins          string // CORS 允许的来源，逗号分隔；生产环境必须明确设置
	TrustedProxy            string // 受信反向代理 CIDR，逗号分隔；为空则仅信任 RemoteAddr
	RetentionDays           int    // 数据保留天数，0=不清理
	PollIntervalSec         int    // 轮询间隔秒数，0=默认3秒
	OfflineThresholdMinutes int    // 离线检测阈值（分钟），默认10，最小1
	RetentionBatchSize      int    // 数据清理每批次最大行数，默认10000
	LoginRateLimit          int    // 每分钟每IP最大登录尝试次数，默认10
}

// ErrNoSecret is returned when the JWT secret is the default and DEV_MODE is not enabled.
var ErrNoSecret = errors.New("XMECO_JWT_SECRET is using the default value – refusing to start in non-dev mode. Set XMECO_JWT_SECRET or XMECO_DEV_MODE=true")

// ErrDefaultDBPassword is returned when the DB password is the default and DEV_MODE is not enabled.
var ErrDefaultDBPassword = errors.New("XMECO_DB_PASSWORD is using the default value – refusing to start in non-dev mode. Set XMECO_DB_PASSWORD or XMECO_DEV_MODE=true")

const defaultJWTSecret = "xmeco-dev-secret-change-in-production" // DEV ONLY — 生产环境必须设置 XMECO_JWT_SECRET
const defaultDBPassword = "xmeco123"                             // DEV ONLY — 生产环境必须设置 XMECO_DB_PASSWORD

func Load() (*Config, error) {
	devMode := strings.EqualFold(getEnv("XMECO_DEV_MODE", ""), "true")
	jwtSecret := getEnv("XMECO_JWT_SECRET", defaultJWTSecret)
	if jwtSecret == defaultJWTSecret && !devMode {
		return nil, ErrNoSecret
	}
	dbPassword := getEnv("XMECO_DB_PASSWORD", defaultDBPassword)
	if dbPassword == defaultDBPassword && !devMode {
		return nil, ErrDefaultDBPassword
	}
	allowedOrigins := getEnv("XMECO_ALLOWED_ORIGINS", "*")
	trustedProxy := getEnv("XMECO_TRUSTED_PROXY", "")
	offlineThreshold := getEnvInt("XMECO_OFFLINE_THRESHOLD_MINUTES", 10)
	if offlineThreshold < 1 {
		slog.Warn("OfflineThresholdMinutes must be >= 1, using default 10", "value", offlineThreshold)
		offlineThreshold = 10
	}
	return &Config{
		DBHost:                  getEnv("XMECO_DB_HOST", "localhost"),
		DBPort:                  getEnv("XMECO_DB_PORT", "5432"),
		DBUser:                  getEnv("XMECO_DB_USER", "postgres"),
		DBPassword:              dbPassword,
		DBName:                  getEnv("XMECO_DB_NAME", "xmeco"),
		DBSSLMode:               getEnv("XMECO_DB_SSLMODE", "disable"),
		ServerPort:              getEnv("XMECO_SERVER_PORT", "9090"),
		JWTSecret:               jwtSecret,
		AllowedOrigins:          allowedOrigins,
		TrustedProxy:            trustedProxy,
		RetentionDays:           getEnvInt("XMECO_RETENTION_DAYS", 730),
		PollIntervalSec:         getEnvInt("XMECO_POLL_INTERVAL_SEC", 3),
		OfflineThresholdMinutes: offlineThreshold,
		RetentionBatchSize:      getEnvInt("XMECO_RETENTION_BATCH_SIZE", 10000),
		LoginRateLimit:          getEnvInt("XMECO_LOGIN_RATE_LIMIT", 10),
	}, nil
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil || i < 0 {
		slog.Warn("invalid env int, using fallback", "key", key, "value", v, "fallback", fallback)
		return fallback
	}
	return i
}

func (c *Config) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, url.QueryEscape(c.DBPassword), c.DBHost, c.DBPort, c.DBName, c.DBSSLMode)
}

// MaskedDSN returns a DSN string with the password replaced by "***" for safe logging.
func (c *Config) MaskedDSN() string {
	return fmt.Sprintf("postgres://%s:***@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode)
}

// TrustedProxyCIDRs returns the parsed list of trusted reverse proxy CIDRs.
func (c *Config) TrustedProxyCIDRs() []string {
	if c.TrustedProxy == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(c.TrustedProxy, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
