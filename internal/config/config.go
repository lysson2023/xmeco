package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	ServerPort string
	JWTSecret  string
	RedisAddr  string
}

func Load() *Config {
	return &Config{
		DBHost:     getEnv("XMECO_DB_HOST", "localhost"),
		DBPort:     getEnv("XMECO_DB_PORT", "5432"),
		DBUser:     getEnv("XMECO_DB_USER", "postgres"),
		DBPassword: getEnv("XMECO_DB_PASSWORD", "xmeco123"), // TODO: 生产环境移除默认值
		DBName:     getEnv("XMECO_DB_NAME", "xmeco"),
		ServerPort: getEnv("XMECO_SERVER_PORT", "9090"),
		JWTSecret:  getEnv("XMECO_JWT_SECRET", "xmeco-dev-secret-change-in-production"),
		RedisAddr:  getEnv("XMECO_REDIS_ADDR", "localhost:6379"),
	}
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
