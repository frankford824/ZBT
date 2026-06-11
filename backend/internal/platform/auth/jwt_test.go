package auth

import (
	"strings"
	"testing"
	"time"
)

func TestJWTSignParseAndReject(t *testing.T) {
	token, err := SignJWT("secret", Claims{
		UserID:    "user-1",
		TenantID:  "tenant-1",
		RoleID:    "role-1",
		RoleCode:  "company_admin",
		Roles:     []string{"company_admin"},
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
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
