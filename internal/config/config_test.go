package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{
		"XMECO_DB_HOST", "XMECO_DB_PORT", "XMECO_DB_USER",
		"XMECO_DB_PASSWORD", "XMECO_DB_NAME", "XMECO_SERVER_PORT",
		"XMECO_JWT_SECRET",
		"XMECO_RETENTION_DAYS", "XMECO_POLL_INTERVAL_SEC",
	} {
		os.Unsetenv(key)
	}
	// JWT secret is now mandatory — set a test value to avoid os.Exit(1).
	os.Setenv("XMECO_JWT_SECRET", "test-secret")
	defer os.Unsetenv("XMECO_JWT_SECRET")

	cfg, err := Load()

	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.DBHost != "localhost" {
		t.Errorf("DBHost = %q, want localhost", cfg.DBHost)
	}
	if cfg.DBPort != "5432" {
		t.Errorf("DBPort = %q, want 5432", cfg.DBPort)
	}
	if cfg.DBUser != "postgres" {
		t.Errorf("DBUser = %q, want postgres", cfg.DBUser)
	}
	if cfg.DBName != "xmeco" {
		t.Errorf("DBName = %q, want xmeco", cfg.DBName)
	}
	if cfg.ServerPort != "9090" {
		t.Errorf("ServerPort = %q, want 9090", cfg.ServerPort)
	}
	if cfg.JWTSecret != "test-secret" {
		t.Errorf("JWTSecret = %q, want test-secret", cfg.JWTSecret)
	}
	if cfg.RetentionDays != 730 {
		t.Errorf("RetentionDays = %d, want 730", cfg.RetentionDays)
	}
	if cfg.PollIntervalSec != 3 {
		t.Errorf("PollIntervalSec = %d, want 3", cfg.PollIntervalSec)
	}
}

func TestLoadCustom(t *testing.T) {
	os.Setenv("XMECO_DB_HOST", "db.example.com")
	os.Setenv("XMECO_DB_PORT", "5433")
	os.Setenv("XMECO_DB_NAME", "xmeco_test")
	os.Setenv("XMECO_SERVER_PORT", "8080")
	os.Setenv("XMECO_JWT_SECRET", "test-secret")
	defer func() {
		for _, key := range []string{
			"XMECO_DB_HOST", "XMECO_DB_PORT", "XMECO_DB_NAME", "XMECO_SERVER_PORT", "XMECO_JWT_SECRET",
		} {
			os.Unsetenv(key)
		}
	}()

	cfg, err := Load()

	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if cfg.DBHost != "db.example.com" {
		t.Errorf("DBHost = %q, want db.example.com", cfg.DBHost)
	}
	if cfg.DBPort != "5433" {
		t.Errorf("DBPort = %q, want 5433", cfg.DBPort)
	}
	if cfg.DBName != "xmeco_test" {
		t.Errorf("DBName = %q, want xmeco_test", cfg.DBName)
	}
	if cfg.ServerPort != "8080" {
		t.Errorf("ServerPort = %q, want 8080", cfg.ServerPort)
	}
}

func TestDSN(t *testing.T) {
	cfg := &Config{
		DBHost: "localhost", DBPort: "5432", DBUser: "postgres",
		DBPassword: "secret", DBName: "xmeco", DBSSLMode: "disable",
	}
	dsn := cfg.DSN()
	expected := "postgres://postgres:secret@localhost:5432/xmeco?sslmode=disable"
	if dsn != expected {
		t.Errorf("DSN = %q, want %q", dsn, expected)
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("XMECO_TEST_KEY", "testval")
	defer os.Unsetenv("XMECO_TEST_KEY")

	// Set value
	if v := getEnv("XMECO_TEST_KEY", "fallback"); v != "testval" {
		t.Errorf("getEnv = %q, want testval", v)
	}

	// Fallback
	if v := getEnv("XMECO_NONEXISTENT", "default"); v != "default" {
		t.Errorf("getEnv fallback = %q, want default", v)
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("XMECO_TEST_INT", "90")
	defer os.Unsetenv("XMECO_TEST_INT")

	if v := getEnvInt("XMECO_TEST_INT", 365); v != 90 {
		t.Errorf("getEnvInt set = %d, want 90", v)
	}
	if v := getEnvInt("XMECO_NONEXISTENT", 365); v != 365 {
		t.Errorf("getEnvInt fallback = %d, want 365", v)
	}
}

func TestGetEnvIntInvalid(t *testing.T) {
	os.Setenv("XMECO_BAD_INT", "abc")
	defer os.Unsetenv("XMECO_BAD_INT")

	if v := getEnvInt("XMECO_BAD_INT", 365); v != 365 {
		t.Errorf("getEnvInt invalid = %d, want fallback 365", v)
	}
}

func TestGetEnvIntZeroRetention(t *testing.T) {
	os.Setenv("XMECO_RETENTION_DAYS", "0")
	os.Setenv("XMECO_JWT_SECRET", "test-secret")
	defer func() {
		os.Unsetenv("XMECO_RETENTION_DAYS")
		os.Unsetenv("XMECO_JWT_SECRET")
	}()

	cfg, err := Load()

	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.RetentionDays != 0 {
		t.Errorf("RetentionDays = %d, want 0 (disabled)", cfg.RetentionDays)
	}
}
