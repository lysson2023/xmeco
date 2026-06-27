package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
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
	// JWT secret and DB password are now mandatory — set test values to avoid startup refusal.
	os.Setenv("XMECO_JWT_SECRET", "test-secret")
	os.Setenv("XMECO_DB_PASSWORD", "test-db-password")
	defer func() {
		os.Unsetenv("XMECO_JWT_SECRET")
		os.Unsetenv("XMECO_DB_PASSWORD")
	}()

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
	os.Setenv("XMECO_DB_PASSWORD", "test-db-password")
	defer func() {
		for _, key := range []string{
			"XMECO_DB_HOST", "XMECO_DB_PORT", "XMECO_DB_NAME", "XMECO_SERVER_PORT", "XMECO_JWT_SECRET", "XMECO_DB_PASSWORD",
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
	os.Setenv("XMECO_DB_PASSWORD", "test-db-password")
	defer func() {
		os.Unsetenv("XMECO_RETENTION_DAYS")
		os.Unsetenv("XMECO_JWT_SECRET")
		os.Unsetenv("XMECO_DB_PASSWORD")
	}()

	cfg, err := Load()

	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.RetentionDays != 0 {
		t.Errorf("RetentionDays = %d, want 0 (disabled)", cfg.RetentionDays)
	}
}

// =============================================================================
// Tier 1 — 安全关键: JWT 密钥检查 (C-01~C-03)
// =============================================================================

func TestLoad_JWTSecurity(t *testing.T) {
	tests := []struct {
		name        string
		jwtSecret   string
		devMode     string
		wantErr     bool
		wantErrIs   error
	}{
		{
			name:      "C-01 生产环境默认密钥应拒绝启动",
			jwtSecret: "",   // 不设置 → 使用 defaultJWTSecret
			devMode:   "",   // 非开发模式
			wantErr:   true,
			wantErrIs: ErrNoSecret,
		},
		{
			name:      "C-02 开发模式默认密钥允许启动",
			jwtSecret: "",   // 不设置 → 使用 defaultJWTSecret
			devMode:   "true",
			wantErr:   false,
		},
		{
			name:      "C-03 自定义密钥正常启动",
			jwtSecret: "my-production-secret-42",
			devMode:   "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清理所有相关环境变量
			os.Unsetenv("XMECO_JWT_SECRET")
			os.Unsetenv("XMECO_DEV_MODE")
			os.Unsetenv("XMECO_DB_PASSWORD")

			if tt.jwtSecret != "" {
				os.Setenv("XMECO_JWT_SECRET", tt.jwtSecret)
			}
			if tt.devMode != "" {
				os.Setenv("XMECO_DEV_MODE", tt.devMode)
			}
			// DB 密码：非 dev 模式需要非默认值
			if tt.devMode != "true" {
				os.Setenv("XMECO_DB_PASSWORD", "test-db-password")
			}
			defer func() {
				os.Unsetenv("XMECO_JWT_SECRET")
				os.Unsetenv("XMECO_DEV_MODE")
				os.Unsetenv("XMECO_DB_PASSWORD")
			}()

			cfg, err := Load()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("error = %v, wantErrIs = %v", err, tt.wantErrIs)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.jwtSecret != "" && cfg.JWTSecret != tt.jwtSecret {
				t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, tt.jwtSecret)
			}
			if tt.devMode == "true" && cfg.JWTSecret == defaultJWTSecret {
				// 开发模式下默认密钥允许，但应当记录为默认值
				t.Logf("DEV_MODE=true with default JWT secret (expected for dev)")
			}
		})
	}
}

// =============================================================================
// Tier 1 — 安全关键: 负数/无效值回退 (C-06)
// =============================================================================

func TestLoad_NegativeRetentionFallback(t *testing.T) {
	os.Setenv("XMECO_RETENTION_DAYS", "-5")
	os.Setenv("XMECO_JWT_SECRET", "test-secret")
	os.Setenv("XMECO_DB_PASSWORD", "test-db-password")
	defer func() {
		os.Unsetenv("XMECO_RETENTION_DAYS")
		os.Unsetenv("XMECO_JWT_SECRET")
		os.Unsetenv("XMECO_DB_PASSWORD")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	// 负数应回退到默认值 730
	if cfg.RetentionDays != 730 {
		t.Errorf("RetentionDays = %d, want 730 (negative value should fallback)", cfg.RetentionDays)
	}
}

// =============================================================================
// Tier 1 — 安全关键: DSN 密码脱敏 (C-10~C-11)
// =============================================================================

func TestMaskedDSN(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		wantContains string
		wantNotContains string
	}{
		{
			name: "C-10 密码被替换为星号",
			cfg: &Config{
				DBHost: "localhost", DBPort: "5432", DBUser: "postgres",
				DBPassword: "secret123", DBName: "xmeco", DBSSLMode: "disable",
			},
			wantContains:    "postgres://postgres:***@",
			wantNotContains: "secret123",
		},
		{
			name: "C-11 密码含特殊字符时被编码",
			cfg: &Config{
				DBHost: "localhost", DBPort: "5432", DBUser: "postgres",
				DBPassword: "a!b@c#d$e%", DBName: "xmeco", DBSSLMode: "disable",
			},
			wantContains:    "postgres://postgres:***@",
			wantNotContains: "a!b@c#d$e%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked := tt.cfg.MaskedDSN()

			if !strings.Contains(masked, tt.wantContains) {
				t.Errorf("MaskedDSN() = %q, want it to contain %q", masked, tt.wantContains)
			}
			if tt.wantNotContains != "" && strings.Contains(masked, tt.wantNotContains) {
				t.Errorf("MaskedDSN() = %q, must NOT contain %q", masked, tt.wantNotContains)
			}
		})
	}
}

// 验证含特殊字符的密码在真实 DSN 中被正确编码 (C-11 补充)
func TestDSN_SpecialCharsPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"空格", "my password"},
		{"URL保留字符", "p@ss:w*rd!/"},
		{"中文字符", "密码123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				DBHost: "localhost", DBPort: "5432", DBUser: "postgres",
				DBPassword: tt.password, DBName: "xmeco", DBSSLMode: "disable",
			}
			dsn := cfg.DSN()

			// 密码不应以明文出现在 DSN 中（但 DSN 本来也不应被日志记录）
			// 重点：DSN 本身必须可以被 url.Parse 解析
			if _, err := url.Parse(dsn); err != nil {
				t.Errorf("DSN() produced unparseable string: %v\nDSN: %s", err, dsn)
			}
			// MaskedDSN 同时也不应泄露密码
			masked := cfg.MaskedDSN()
			if strings.Contains(masked, tt.password) {
				t.Errorf("MaskedDSN() leaks password: %s", masked)
			}
		})
	}
}

// =============================================================================
// Tier 1 — 安全关键: TrustedProxyCIDRs (C-12~C-14)
// =============================================================================

func TestTrustedProxyCIDRs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		want    []string
	}{
		{
			name:    "C-12 空字符串返回nil",
			input:   "",
			wantLen: 0,
			want:    nil,
		},
		{
			name:    "C-13 单个CIDR",
			input:   "10.0.0.0/8",
			wantLen: 1,
			want:    []string{"10.0.0.0/8"},
		},
		{
			name:    "C-14 多个CIDR逗号分隔",
			input:   "10.0.0.0/8, 172.16.0.0/12",
			wantLen: 2,
			want:    []string{"10.0.0.0/8", "172.16.0.0/12"},
		},
		{
			name:    "含多余空格和空项",
			input:   " 10.0.0.0/8 , , 172.16.0.0/12 ",
			wantLen: 2,
			want:    []string{"10.0.0.0/8", "172.16.0.0/12"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{TrustedProxy: tt.input}
			got := cfg.TrustedProxyCIDRs()

			if tt.want == nil {
				if got != nil {
					t.Errorf("TrustedProxyCIDRs() = %v, want nil", got)
				}
				return
			}

			if len(got) != tt.wantLen {
				t.Errorf("len(TrustedProxyCIDRs()) = %d, want %d", len(got), tt.wantLen)
			}
			for i, want := range tt.want {
				if i >= len(got) || got[i] != want {
					t.Errorf("TrustedProxyCIDRs()[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}
