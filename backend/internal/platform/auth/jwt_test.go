package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJWTSignParseAndReject(t *testing.T) {
	issuedAt := time.Now()
	token, err := SignJWT("secret", Claims{
		UserID:     "user-1",
		TenantID:   "tenant-1",
		RoleID:     "role-1",
		RoleCode:   "company_admin",
		Roles:      []string{"company_admin"},
		IssuedAt:   issuedAt.Unix(),
		IssuedAtNS: issuedAt.UnixNano(),
		ExpiresAt:  issuedAt.Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ParseJWT("secret", token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-1" || claims.TenantID != "tenant-1" || claims.RoleCode != "company_admin" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if claims.IssuedAt != issuedAt.Unix() || claims.IssuedAtNS != issuedAt.UnixNano() {
		t.Fatalf("expected issued-at claims to round trip, got %#v", claims)
	}

	tampered := token[:len(token)-1] + "x"
	if _, err := ParseJWT("secret", tampered); err != ErrInvalidToken {
		t.Fatalf("expected invalid token, got %v", err)
	}

	expired, err := SignJWT("secret", Claims{UserID: "user-1", ExpiresAt: time.Now().Add(-time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseJWT("secret", expired); err != ErrExpiredToken {
		t.Fatalf("expected expired token, got %v", err)
	}

	if strings.Count(token, ".") != 2 {
		t.Fatalf("expected jwt with three parts, got %q", token)
	}
}

func TestJWTRejectsMissingExpiration(t *testing.T) {
	token, err := SignJWT("secret", Claims{UserID: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseJWT("secret", token); err != ErrInvalidToken {
		t.Fatalf("expected missing exp to be rejected, got %v", err)
	}
}

func TestJWTRejectsUnsupportedHeaderAlgorithm(t *testing.T) {
	token := signedTokenWithHeader(t, "secret", map[string]string{"alg": "HS384", "typ": "JWT"}, Claims{
		UserID:    "user-1",
		TenantID:  "tenant-1",
		RoleID:    "role-1",
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	if _, err := ParseJWT("secret", token); err != ErrInvalidToken {
		t.Fatalf("expected unsupported alg to be rejected, got %v", err)
	}
}

func TestJWTRejectsTokenAtExpirationBoundary(t *testing.T) {
	token, err := SignJWT("secret", Claims{UserID: "user-1", ExpiresAt: time.Now().Unix()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseJWT("secret", token); err != ErrExpiredToken {
		t.Fatalf("expected token at exp boundary to be expired, got %v", err)
	}
}

func signedTokenWithHeader(t *testing.T, secret string, header map[string]string, claims Claims) string {
	t.Helper()
	headerBytes, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	claimBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimBytes)
	signingInput := encodedHeader + "." + encodedClaims
	return signingInput + "." + sign(secret, signingInput)
}
