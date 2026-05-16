package auth

import (
	"testing"
	"time"
)

func TestSignAndParse(t *testing.T) {
	secret := []byte("test-secret-key-for-jwt-testing")
	userID := "user-123"
	workspaceID := "ws-456"
	role := "admin"

	tok, err := Sign(secret, userID, workspaceID, role, 1*time.Hour)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := Parse(secret, tok)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %q, want %q", claims.UserID, userID)
	}
	if claims.WorkspaceID != workspaceID {
		t.Errorf("WorkspaceID = %q, want %q", claims.WorkspaceID, workspaceID)
	}
	if claims.Role != role {
		t.Errorf("Role = %q, want %q", claims.Role, role)
	}
	if claims.ID == "" {
		t.Error("expected jti to be set")
	}
}

func TestExpiredToken(t *testing.T) {
	secret := []byte("test-secret-key")
	tok, err := Sign(secret, "user-1", "ws-1", "viewer", -1*time.Second)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	_, err = Parse(secret, tok)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestInvalidSignature(t *testing.T) {
	secret := []byte("test-secret-key")
	tok, err := Sign(secret, "user-1", "ws-1", "viewer", 1*time.Hour)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	_, err = Parse([]byte("wrong-secret-key"), tok)
	if err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestSignSSEToken(t *testing.T) {
	secret := []byte("sse-secret-key")
	sessionID := "session-abc"
	tok, err := SignSSEToken(secret, "user-1", "ws-1", sessionID)
	if err != nil {
		t.Fatalf("SignSSEToken failed: %v", err)
	}
	claims, err := Parse(secret, tok)
	if err != nil {
		t.Fatalf("Parse SSE token failed: %v", err)
	}
	if claims.Role != "viewer" {
		t.Errorf("expected viewer role, got %q", claims.Role)
	}
	if claims.UserID != "user-1" {
		t.Errorf("expected UserID = %q, got %q", "user-1", claims.UserID)
	}
	if claims.ID == "" {
		t.Error("expected jti to be set on SSE token")
	}
}
