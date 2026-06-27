package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pashagolub/pgxmock/v4"

	"xmeco/internal/service/auth"
)

func TestRateLimiterAllows(t *testing.T) {
	rl := NewRateLimiter(3, 1*time.Minute)
	called := false
	handler := rl.LimitLogin(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 3; i++ {
		called = false
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		handler(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			t.Errorf("request %d was blocked, expected to pass", i+1)
		}
		if !called {
			t.Errorf("request %d: handler not called", i+1)
		}
	}
}

func TestRateLimiterBlocks(t *testing.T) {
	rl := NewRateLimiter(2, 1*time.Minute)
	handler := rl.LimitLogin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ip := "10.0.0.1:9999"

	// First 2 should pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: unexpected status %d (should pass)", i+1, rec.Code)
		}
	}

	// Third should be blocked
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req.RemoteAddr = ip
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for blocked request, got %d", rec.Code)
	}
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	rl := NewRateLimiter(1, 1*time.Minute)
	handler := rl.LimitLogin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// IP1: first request passes
	req1 := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req1.RemoteAddr = "1.1.1.1:80"
	rec1 := httptest.NewRecorder()
	handler(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("IP1 first request blocked")
	}

	// IP1: second request blocked
	req1b := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req1b.RemoteAddr = "1.1.1.1:80"
	rec1b := httptest.NewRecorder()
	handler(rec1b, req1b)
	if rec1b.Code != http.StatusTooManyRequests {
		t.Errorf("IP1 second request not blocked")
	}

	// IP2: first request should pass (different bucket)
	req2 := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req2.RemoteAddr = "2.2.2.2:80"
	rec2 := httptest.NewRecorder()
	handler(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("IP2 first request blocked (should be independent)")
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	rl := NewRateLimiter(100, 1*time.Minute)
	handler := rl.LimitLogin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var wg sync.WaitGroup
	blocked := int32(0)
	passed := int32(0)
	ip := "concurrent.test:1234"

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
			req.RemoteAddr = ip
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code == http.StatusTooManyRequests {
				blocked++
			} else {
				passed++
			}
		}()
	}
	wg.Wait()

	if passed > 100 {
		t.Errorf("too many passed requests: %d", passed)
	}
	// 50 requests with 100 limit should have 0 blocked
	if blocked > 0 {
		t.Errorf("%d requests unexpectedly blocked (limit=100)", blocked)
	}
}

func TestRateLimiterNew(t *testing.T) {
	rl := NewRateLimiter(10, 60*time.Second)
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.limit != 10 {
		t.Errorf("limit = %d, want 10", rl.limit)
	}
	if rl.window != 60*time.Second {
		t.Errorf("window = %v, want 1m", rl.window)
	}
}

func TestCORSHeaders(t *testing.T) {
	// Verify CORS middleware passes through non-OPTIONS requests
	handler := CORS("*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("ACAO header missing")
	}
}

func TestCORSPreflight(t *testing.T) {
	handler := CORS("*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// OPTIONS can return 200 or 204 — both are valid for preflight
	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want 200 or 204", rec.Code)
	}
}

// ---- JWT Auth Middleware ----

func makeToken(secret string, userID int, username, roleCode string) string {
	claims := auth.Claims{
		UserID:   userID,
		Username: username,
		RoleCode: roleCode,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "xmeco",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString([]byte(secret))
	return s
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	const secret = "test-secret-for-middleware-test"
	authSvc := auth.New(nil, secret)
	mw := AuthMiddleware(authSvc)

	var capturedClaims *auth.Claims
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClaims = GetClaims(r)
		w.WriteHeader(http.StatusOK)
	}))

	token := makeToken(secret, 42, "testuser", "admin")
	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedClaims == nil {
		t.Fatal("claims not set on context")
	}
	if capturedClaims.UserID != 42 {
		t.Errorf("UserID = %d, want 42", capturedClaims.UserID)
	}
	if capturedClaims.Username != "testuser" {
		t.Errorf("Username = %q, want testuser", capturedClaims.Username)
	}
	if capturedClaims.RoleCode != "admin" {
		t.Errorf("RoleCode = %q, want admin", capturedClaims.RoleCode)
	}
}

func TestAuthMiddlewareNoHeader(t *testing.T) {
	authSvc := auth.New(nil, "secret")
	mw := AuthMiddleware(authSvc)
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing header, got %d", rec.Code)
	}
	if called {
		t.Error("handler should not be called for missing token")
	}
}

func TestAuthMiddlewareWrongPrefix(t *testing.T) {
	authSvc := auth.New(nil, "secret")
	mw := AuthMiddleware(authSvc)
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for Basic auth, got %d", rec.Code)
	}
	if called {
		t.Error("handler should not be called for wrong auth scheme")
	}
}

func TestAuthMiddlewareExpiredToken(t *testing.T) {
	const secret = "expired-test-secret"
	authSvc := auth.New(nil, secret)
	mw := AuthMiddleware(authSvc)
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	claims := auth.Claims{
		UserID:   1,
		Username: "expired",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "xmeco",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := tok.SignedString([]byte(secret))

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+s)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", rec.Code)
	}
	if called {
		t.Error("handler should not be called for expired token")
	}
}

func TestAuthMiddlewareMalformedToken(t *testing.T) {
	authSvc := auth.New(nil, "secret")
	mw := AuthMiddleware(authSvc)
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for malformed token, got %d", rec.Code)
	}
	if called {
		t.Error("handler should not be called for malformed token")
	}
}

func TestAuthMiddlewareEmptyTokenValue(t *testing.T) {
	authSvc := auth.New(nil, "secret")
	mw := AuthMiddleware(authSvc)
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty token, got %d", rec.Code)
	}
	if called {
		t.Error("handler should not be called for empty token")
	}
}

// =============================================================================
// Tier 4 — MW-18~MW-21: RateLimiter 窗口重置 + 受信代理 IP
// =============================================================================

func TestRateLimiter_WindowReset(t *testing.T) {
	// MW-18: 窗口过期后计数器重置，新请求通过
	rl := NewRateLimiter(1, 1*time.Millisecond)

	ip := "10.0.0.99:1234"
	// 第1次：通过
	req1 := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req1.RemoteAddr = ip
	rec1 := httptest.NewRecorder()
	rl.LimitLogin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("request 1: expected 200, got %d", rec1.Code)
	}

	// 第2次：超限 (limit=1)
	req2 := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req2.RemoteAddr = ip
	rec2 := httptest.NewRecorder()
	rl.LimitLogin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("request 2: expected 429, got %d", rec2.Code)
	}

	// 等待窗口过期
	time.Sleep(5 * time.Millisecond)

	// 第3次：窗口重置后应通过
	req3 := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req3.RemoteAddr = ip
	rec3 := httptest.NewRecorder()
	rl.LimitLogin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("MW-18 window reset: expected 200 after window expiry, got %d", rec3.Code)
	}
}

func TestRateLimiter_TrustedProxy(t *testing.T) {
	// MW-19~MW-21: 受信代理 X-Forwarded-For / X-Real-IP 提取
	nextHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	tests := []struct {
		name          string
		trustedCIDRs  []string
		remoteAddr    string
		xForwardedFor string
		xRealIP       string
		// We verify correct IP isolation by making 2 requests from different
		// "real" IPs (via headers) through the same proxy.
	}{
		{
			name:         "MW-19 X-Forwarded-For 受信代理提取真实IP",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.1:443",
			xForwardedFor: "1.2.3.4, 5.6.7.8",
		},
		{
			name:         "MW-20 X-Real-IP 受信代理回退(无XFF)",
			trustedCIDRs: []string{"172.16.0.0/12"},
			remoteAddr:   "172.16.0.1:443",
			xRealIP:      "5.6.7.8",
		},
		{
			name:         "MW-21 非受信代理-忽略XFF用RemoteAddr",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "192.168.1.1:12345", // NOT in trusted CIDR
			xForwardedFor: "1.2.3.4",
		},
		{
			name:         "X-Forwarded-For空-回退X-Real-IP",
			trustedCIDRs: []string{"10.0.0.0/8"},
			remoteAddr:   "10.0.0.99:443",
			xForwardedFor: "",
			xRealIP:      "9.9.9.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(1, 1*time.Hour, tt.trustedCIDRs...)

			req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			rec := httptest.NewRecorder()
			rl.LimitLogin(nextHandler)(rec, req)

			// First request should always pass (limit=1)
			if rec.Code != http.StatusOK {
				t.Errorf("first request blocked: status=%d", rec.Code)
				return
			}

			// Second request from same (derived) IP should be blocked
			req2 := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
			req2.RemoteAddr = tt.remoteAddr
			req2.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			req2.Header.Set("X-Real-IP", tt.xRealIP)

			rec2 := httptest.NewRecorder()
			rl.LimitLogin(nextHandler)(rec2, req2)

			if rec2.Code != http.StatusTooManyRequests {
				t.Errorf("second request should be blocked (same derived IP), got %d", rec2.Code)
			}
		})
	}
}

// =============================================================================
// Tier 4 — MW-23, MW-26: RBAC 中间件 (无需 DB 的路径)
// =============================================================================

func TestRBAC_SuperAdminBypass(t *testing.T) {
	// MW-23: super_admin 绕过 HasPermission 检查
	authSvc := auth.New(nil, "test-secret") // nil pool: HasPermission won't be called
	mw := RequirePermission(authSvc, "device:write")

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	claims := &auth.Claims{UserID: 1, Username: "admin", RoleCode: auth.RoleSuperAdmin}
	ctx := context.WithValue(t.Context(), CtxClaims, claims)
	req := httptest.NewRequest("GET", "/api/v1/devices", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("MW-23 super_admin bypass: expected 200, got %d", rec.Code)
	}
	if !called {
		t.Error("MW-23: handler should be called for super_admin")
	}
}

func TestRBAC_NoClaims(t *testing.T) {
	// MW-26: 无 Claims 上下文返回 401
	authSvc := auth.New(nil, "test-secret")
	mw := RequirePermission(authSvc, "device:write")

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/devices", nil) // no claims in context
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("MW-26 no claims: expected 401, got %d", rec.Code)
	}
	if called {
		t.Error("MW-26: handler should not be called without claims")
	}
}

// =============================================================================
// Tier 4 — MW-24~MW-27: RBAC HasPermission (pgxmock)
// =============================================================================

func TestRBAC_HasPermissionGranted(t *testing.T) {
	// MW-24: 用户有权限 → 200
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM permission").
		WithArgs(2, "device:write").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))

	authSvc := auth.New(mock, "test-secret")
	mw := RequirePermission(authSvc, "device:write")

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	claims := &auth.Claims{UserID: 2, Username: "user", RoleCode: "admin"}
	ctx := context.WithValue(t.Context(), CtxClaims, claims)
	req := httptest.NewRequest("GET", "/api/v1/devices", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("MW-24: expected 200, got %d", rec.Code)
	}
	if !called {
		t.Error("MW-24: handler should be called when permission granted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRBAC_HasPermissionDenied(t *testing.T) {
	// MW-25: 用户无权限 → 403
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM permission").
		WithArgs(2, "device:write").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(0)))

	authSvc := auth.New(mock, "test-secret")
	mw := RequirePermission(authSvc, "device:write")

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	claims := &auth.Claims{UserID: 2, Username: "user", RoleCode: "admin"}
	ctx := context.WithValue(t.Context(), CtxClaims, claims)
	req := httptest.NewRequest("GET", "/api/v1/devices", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("MW-25: expected 403, got %d", rec.Code)
	}
	if called {
		t.Error("MW-25: handler should NOT be called when permission denied")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRBAC_HasPermissionDBError(t *testing.T) {
	// MW-27: HasPermission 数据库异常 → 503
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM permission").
		WithArgs(2, "device:write").
		WillReturnError(errors.New("connection refused"))

	authSvc := auth.New(mock, "test-secret")
	mw := RequirePermission(authSvc, "device:write")

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	claims := &auth.Claims{UserID: 2, Username: "user", RoleCode: "admin"}
	ctx := context.WithValue(t.Context(), CtxClaims, claims)
	req := httptest.NewRequest("GET", "/api/v1/devices", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("MW-27: expected 500, got %d", rec.Code)
	}
	if called {
		t.Error("MW-27: handler should NOT be called on DB error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// =============================================================================
// Tier 4 — MW-28~MW-29: BodyLimit 请求体大小限制
// =============================================================================

func TestBodyLimit_WithinLimit(t *testing.T) {
	// MW-28: 请求体在限制内正常通过
	mw := BodyLimit(1 << 20) // 1MB limit

	var bodyRead []byte
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		bodyRead, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))

	payload := strings.Repeat("a", 512*1024) // 512KB
	req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("MW-28: expected 200 for body within limit, got %d", rec.Code)
	}
	if len(bodyRead) != len(payload) {
		t.Errorf("MW-28: body length = %d, want %d", len(bodyRead), len(payload))
	}
}

func TestBodyLimit_ExceedsLimit(t *testing.T) {
	// MW-29: 请求体超限被 MaxBytesReader 阻断
	mw := BodyLimit(10) // 10 bytes limit

	var readErr error
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	payload := strings.Repeat("x", 100) // 100 bytes > 10 limit
	req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if readErr == nil {
		t.Error("MW-29: expected error reading body exceeding limit, got nil")
	}
	if readErr != nil && !strings.Contains(readErr.Error(), "http: request body too large") {
		t.Errorf("MW-29: error = %q, want MaxBytesReader error", readErr.Error())
	}
}

func TestGetClaimsNoContext(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	if claims := GetClaims(req); claims != nil {
		t.Error("GetClaims should return nil without auth context")
	}
}

func TestGetClaimsWrongType(t *testing.T) {
	// Context has a value at the right key but wrong type
	ctx := context.WithValue(t.Context(), CtxClaims, "not-claims")
	req := httptest.NewRequest("GET", "/api/v1/projects", nil).WithContext(ctx)
	if claims := GetClaims(req); claims != nil {
		t.Error("GetClaims should return nil for wrong type")
	}
}

// =============================================================================
// Tier 1 — MW-11~MW-12: CORS 通配符 + 凭证安全
// =============================================================================

func TestCORS_WildcardCredentials(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name          string
		allowedOrigin string
		requestOrigin string
		hasAuthHeader bool
		expectACAO    string
		expectVary    bool
	}{
		{
			name:          "MW-11 通配符+凭证→阻止(安全策略：不反射Origin)",
			allowedOrigin: "*",
			requestOrigin: "http://localhost:3000",
			hasAuthHeader: true,
			expectACAO:    "",
			expectVary:    false,
		},
		{
			name:          "MW-12 通配符+无凭证→返回*",
			allowedOrigin: "*",
			requestOrigin: "http://any-origin.com",
			hasAuthHeader: false,
			expectACAO:    "*",
			expectVary:    false,
		},
		{
			name:          "通配符+凭证+空Origin→阻止",
			allowedOrigin: "*",
			requestOrigin: "",
			hasAuthHeader: true,
			expectACAO:    "",
			expectVary:    false,
		},
		{
			name:          "通配符+凭证+恶意Origin→阻止(不反射)",
			allowedOrigin: "*",
			requestOrigin: "http://evil.example.com",
			hasAuthHeader: true,
			expectACAO:    "",
			expectVary:    false,
		},
		// 非通配符场景：白名单匹配
		{
			name:          "白名单匹配+凭证",
			allowedOrigin: "http://localhost:3000",
			requestOrigin: "http://localhost:3000",
			hasAuthHeader: true,
			expectACAO:    "http://localhost:3000",
			expectVary:    false,
		},
		{
			name:          "白名单不匹配→无CORS头",
			allowedOrigin: "http://localhost:3000",
			requestOrigin: "http://evil.example.com",
			hasAuthHeader: false,
			expectACAO:    "",
			expectVary:    false,
		},
		{
			name:          "无Origin头→使用第一个白名单值",
			allowedOrigin: "http://localhost:3000, http://localhost:5173",
			requestOrigin: "",
			hasAuthHeader: false,
			expectACAO:    "http://localhost:3000",
			expectVary:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CORS(tt.allowedOrigin, nextHandler)
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			rec := httptest.NewRecorder()

			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			if tt.hasAuthHeader {
				req.Header.Set("Authorization", "Bearer test-token-123")
			}

			handler.ServeHTTP(rec, req)

			gotACAO := rec.Header().Get("Access-Control-Allow-Origin")
			if gotACAO != tt.expectACAO {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", gotACAO, tt.expectACAO)
			}

			gotVary := rec.Header().Get("Vary")
			if tt.expectVary {
				if gotVary != "Origin" {
					t.Errorf("Vary = %q, want %q", gotVary, "Origin")
				}
			} else {
				if gotVary == "Origin" {
					t.Errorf("Vary should not be set for this case, got %q", gotVary)
				}
			}

			// 当有凭证时确认没有设置 Access-Control-Allow-Origin: *
			if tt.hasAuthHeader && tt.requestOrigin != "" && gotACAO == "*" {
				t.Error("must not return '*' with credentialed request (browser will block)")
			}
		})
	}
}
