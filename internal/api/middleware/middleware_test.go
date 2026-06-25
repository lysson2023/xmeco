package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

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
