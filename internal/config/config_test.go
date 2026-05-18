package config

import (
	"os"
	"testing"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("setenv %s: %v", key, err)
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv %s: %v", key, err)
	}
}

func mustSetRequired(t *testing.T) {
	t.Helper()
	setEnv(t, "HUB_JWT_SECRET", "test-jwt-secret-for-config-test")
	setEnv(t, "HUB_DB_ENCRYPTION_KEY", "0000000000000000000000000000000000000000000000000000000000000000")
}

func TestLoad_MissingSecret(t *testing.T) {
	unsetEnv(t, "HUB_JWT_SECRET")
	unsetEnv(t, "HUB_DB_ENCRYPTION_KEY")

	_, err := Load()
	if err == nil {
		t.Error("Load() should return error when HUB_JWT_SECRET is missing")
	}
}

func TestLoad_MissingDBEncryptionKey(t *testing.T) {
	setEnv(t, "HUB_JWT_SECRET", "test-jwt")
	unsetEnv(t, "HUB_DB_ENCRYPTION_KEY")

	_, err := Load()
	if err == nil {
		t.Error("Load() should return error when HUB_DB_ENCRYPTION_KEY is missing")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	mustSetRequired(t)
	defer unsetEnv(t, "HUB_JWT_SECRET")
	defer unsetEnv(t, "HUB_DB_ENCRYPTION_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if string(cfg.JWTSecret) != "test-jwt-secret-for-config-test" {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, "test-jwt-secret-for-config-test")
	}
	if cfg.GRPCAddr != ":9090" {
		t.Errorf("GRPCAddr = %q, want %q", cfg.GRPCAddr, ":9090")
	}
	if cfg.HTTPAddr == "" {
		t.Error("HTTPAddr should have default value")
	}
	if cfg.MySQLHost != "localhost" {
		t.Errorf("MySQLHost = %q, want %q", cfg.MySQLHost, "localhost")
	}
	if cfg.MySQLPort != 3306 {
		t.Errorf("MySQLPort = %d, want %d", cfg.MySQLPort, 3306)
	}
	if cfg.MySQLRootUser != "root" {
		t.Errorf("MySQLRootUser = %q, want %q", cfg.MySQLRootUser, "root")
	}
	if cfg.MySQLPlatformDB != "hub_platform" {
		t.Errorf("MySQLPlatformDB = %q, want %q", cfg.MySQLPlatformDB, "hub_platform")
	}
}
