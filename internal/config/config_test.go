package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any existing env vars for the test
	for _, key := range []string{
		"XMECO_DB_HOST", "XMECO_DB_PORT", "XMECO_DB_USER",
		"XMECO_DB_PASSWORD", "XMECO_DB_NAME", "XMECO_SERVER_PORT",
		"XMECO_JWT_SECRET", "XMECO_WEATHER_API_KEY",
	} {
		os.Unsetenv(key)
	}

	cfg := Load()

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
	if cfg.JWTSecret != "xmeco-dev-secret-change-in-production" {
		t.Errorf("JWTSecret uses default")
	}
}

func TestLoadCustom(t *testing.T) {
	os.Setenv("XMECO_DB_HOST", "db.example.com")
	os.Setenv("XMECO_DB_PORT", "5433")
	os.Setenv("XMECO_DB_NAME", "xmeco_test")
	os.Setenv("XMECO_SERVER_PORT", "8080")
	defer func() {
		for _, key := range []string{
			"XMECO_DB_HOST", "XMECO_DB_PORT", "XMECO_DB_NAME", "XMECO_SERVER_PORT",
		} {
			os.Unsetenv(key)
		}
	}()

	cfg := Load()

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
		DBPassword: "secret", DBName: "xmeco",
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
