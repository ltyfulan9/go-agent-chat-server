package jwtutil

import (
	"testing"
	"time"
)

func TestGenerateAndParseToken(t *testing.T) {
	secret := "test-secret"
	token, err := GenerateToken("user-1", "alice", secret, time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}

	claims, err := ParseToken(token, secret)
	if err != nil {
		t.Fatalf("ParseToken error: %v", err)
	}
	if claims.UserID != "user-1" || claims.Username != "alice" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseTokenRejectsWrongSecret(t *testing.T) {
	token, err := GenerateToken("user-1", "alice", "secret-a", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}

	if _, err := ParseToken(token, "secret-b"); err == nil {
		t.Fatal("expected wrong secret to be rejected")
	}
}

func TestParseTokenRejectsExpiredToken(t *testing.T) {
	token, err := GenerateToken("user-1", "alice", "secret", -time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken error: %v", err)
	}

	if _, err := ParseToken(token, "secret"); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}
