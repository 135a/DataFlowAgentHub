package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 存储从环境变量加载的进程配置。
type Config struct {
	HTTPAddr     string
	GRPCAddr     string
	DatabaseURL  string
	RedisAddr    string
	JWTSecret    []byte
	SeedEmail    string
	SeedPassword string
	NL2SQLTarget string
	LLMBaseURL   string
	LLMModel     string
	LLMAPIKey    string
	LLMTimeout   time.Duration
	ApprovalTTL  time.Duration
	QueryMaxRows int32
	QueryTimeout time.Duration

	SchemaCacheTTL           time.Duration
	SchemaMaxTables          int32
	SchemaMaxColumnsPerTable int32
	DBEncryptionKey          string
	ReportsDir               string
	OTelExporterEndpoint     string
	Env                      string
	NATSURL                  string
	RateLimitFailClosed      bool

	// mTLS 证书路径（为空时使用 insecure）
	GRPCCACert     string
	GRPCServerCert string
	GRPCServerKey  string
	GRPCClientCert string
	GRPCClientKey  string
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func mustDur(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func mustInt32(key string, def int32) int32 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return def
	}
	return int32(n)
}

// Load 从环境变量读取配置。
func Load() (*Config, error) {
	sec := os.Getenv("HUB_JWT_SECRET")
	if sec == "" {
		return nil, fmt.Errorf("HUB_JWT_SECRET is required")
	}
	dbEncKey := os.Getenv("HUB_DB_ENCRYPTION_KEY")
	if dbEncKey == "" {
		return nil, fmt.Errorf("HUB_DB_ENCRYPTION_KEY is required (32-byte hex-encoded key)")
	}
	c := &Config{
		HTTPAddr:                 getenv("HUB_HTTP_ADDR", ":8080"),
		GRPCAddr:                 getenv("HUB_GRPC_ADDR", ":9090"),
		DatabaseURL:              getenv("HUB_DATABASE_URL", "postgres://hub:hub@localhost:5432/hub?sslmode=disable"),
		RedisAddr:                getenv("HUB_REDIS_ADDR", "localhost:6379"),
		JWTSecret:                []byte(sec),
		SeedEmail:                getenv("HUB_SEED_EMAIL", "admin@demo.local"),
		SeedPassword:             getenv("HUB_SEED_PASSWORD", "changeme"),
		NL2SQLTarget:             getenv("HUB_NL2SQL_TARGET", "localhost:50051"),
		LLMBaseURL:               getenv("HUB_LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMModel:                 getenv("HUB_LLM_MODEL", "gpt-4o-mini"),
		LLMAPIKey:                os.Getenv("HUB_LLM_API_KEY"),
		LLMTimeout:               mustDur("HUB_LLM_TIMEOUT", 60*time.Second),
		ApprovalTTL:              mustDur("HUB_APPROVAL_TTL", 24*time.Hour),
		QueryMaxRows:             mustInt32("HUB_QUERY_MAX_ROWS", 500),
		QueryTimeout:             mustDur("HUB_QUERY_TIMEOUT", 30*time.Second),
		SchemaCacheTTL:           mustDur("HUB_SCHEMA_CACHE_TTL", 300*time.Second),
		SchemaMaxTables:          mustInt32("HUB_SCHEMA_MAX_TABLES", 50),
		SchemaMaxColumnsPerTable: mustInt32("HUB_SCHEMA_MAX_COLUMNS_PER_TABLE", 100),
		DBEncryptionKey:          dbEncKey,
		ReportsDir:               getenv("HUB_REPORTS_DIR", os.TempDir()+"/hub-reports/"),
		OTelExporterEndpoint:     os.Getenv("HUB_OTEL_EXPORTER_ENDPOINT"),
		Env:                      getenv("HUB_ENV", "development"),
		NATSURL:                  getenv("HUB_NATS_URL", "nats://localhost:4222"),
		RateLimitFailClosed:      os.Getenv("HUB_RATELIMIT_FAIL_CLOSED") == "true",
		GRPCCACert:               os.Getenv("HUB_GRPC_CA_CERT"),
		GRPCServerCert:           os.Getenv("HUB_GRPC_SERVER_CERT"),
		GRPCServerKey:            os.Getenv("HUB_GRPC_SERVER_KEY"),
		GRPCClientCert:           os.Getenv("HUB_GRPC_CLIENT_CERT"),
		GRPCClientKey:            os.Getenv("HUB_GRPC_CLIENT_KEY"),
	}
	return c, nil
}
