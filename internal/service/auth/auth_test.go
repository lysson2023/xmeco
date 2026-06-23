package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("admin123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if len(hash) < 40 {
		t.Errorf("hash too short: %d chars", len(hash))
	}

	// Same password should produce different hash (bcrypt salt)
	hash2, _ := HashPassword("admin123")
	if hash == hash2 {
		t.Error("bcrypt should produce different hashes for same input due to salt")
	}
}

func TestHashPasswordEmpty(t *testing.T) {
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword empty failed: %v", err)
	}
	if len(hash) < 40 {
		t.Errorf("empty password hash too short: %d chars", len(hash))
	}
}

func TestValidateToken(t *testing.T) {
	secret := "test-secret-key-32bytes-minimum!!"
	s := New(nil, secret)

	// Create a valid token
	claims := Claims{
		UserID:   1,
		Username: "admin",
		RoleCode: "super_admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "xmeco",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// Validate
	parsed, err := s.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if parsed.UserID != 1 {
		t.Errorf("userID = %d, want 1", parsed.UserID)
	}
	if parsed.Username != "admin" {
		t.Errorf("username = %q, want admin", parsed.Username)
	}
	if parsed.RoleCode != "super_admin" {
		t.Errorf("roleCode = %q, want super_admin", parsed.RoleCode)
	}
	if parsed.Issuer != "xmeco" {
		t.Errorf("issuer = %q, want xmeco", parsed.Issuer)
	}
}

func TestValidateTokenExpired(t *testing.T) {
	secret := "another-test-secret-key-32bytes"
	s := New(nil, secret)

	// Create expired token
	claims := Claims{
		UserID:   1,
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "xmeco",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))

	_, err := s.ValidateToken(tokenStr)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestValidateTokenWrongSecret(t *testing.T) {
	s := New(nil, "correct-secret-for-signing-only")

	// Sign with a different secret
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:   1,
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			Issuer:    "xmeco",
		},
	})
	tokenStr, _ := token.SignedString([]byte("wrong-secret-key-here"))

	_, err := s.ValidateToken(tokenStr)
	if err == nil {
		t.Error("expected error for wrong secret, got nil")
	}
}

func TestValidateTokenMalformed(t *testing.T) {
	s := New(nil, "test-secret")

	_, err := s.ValidateToken("this.is.not.a.valid.token")
	if err == nil {
		t.Error("expected error for malformed token")
	}
}

func TestValidateTokenEmpty(t *testing.T) {
	s := New(nil, "test-secret")

	_, err := s.ValidateToken("")
	if err == nil {
		t.Error("expected error for empty token")
	}
}
