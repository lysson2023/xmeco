package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	ServerPort      string
	JWTSecret       string
	AllowedOrigins  string // CORS 允许的来源，逗号分隔；默认 "*"
	RetentionDays   int    // 数据保留天数，0=不清理
	PollIntervalSec int    // 轮询间隔秒数，0=默认3秒
}

const defaultJWTSecret = "xmeco-dev-secret-change-in-production"

func Load() *Config {
	jwtSecret := getEnv("XMECO_JWT_SECRET", defaultJWTSecret)
	if jwtSecret == defaultJWTSecret {
		slog.Error("XMECO_JWT_SECRET is using the default value – refusing to start. Set XMECO_JWT_SECRET for production.")
		os.Exit(1)
	}
	return &Config{
		DBHost:          getEnv("XMECO_DB_HOST", "localhost"),
		DBPort:          getEnv("XMECO_DB_PORT", "5432"),
		DBUser:          getEnv("XMECO_DB_USER", "postgres"),
		DBPassword:      getEnv("XMECO_DB_PASSWORD", "xmeco123"),
		DBName:          getEnv("XMECO_DB_NAME", "xmeco"),
		ServerPort:      getEnv("XMECO_SERVER_PORT", "9090"),
		JWTSecret:       jwtSecret,
		AllowedOrigins:  getEnv("XMECO_ALLOWED_ORIGINS", "*"),
		RetentionDays:   getEnvInt("XMECO_RETENTION_DAYS", 730),
		PollIntervalSec: getEnvInt("XMECO_POLL_INTERVAL_SEC", 3),
	}
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil || i < 0 {
		return fallback
	}
	return i
}

func (c *Config) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
