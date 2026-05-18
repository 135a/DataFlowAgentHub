package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	// GRPCCACert 用于存储CA证书的路径或内容，用于验证gRPC通信中的服务器和客户端证书
	GRPCCACert string
	// GRPCServerCert 用于存储gRPC服务器证书的路径或内容，用于服务器身份验证
	GRPCServerCert string
	// GRPCServerKey 用于存储gRPC服务器私钥的路径或内容，与服务器证书配对使用
	GRPCServerKey string
	// GRPCClientCert 用于存储gRPC客户端证书的路径或内容，用于客户端身份验证
	GRPCClientCert string
	// GRPCClientKey 用于存储gRPC客户端私钥的路径或内容，与客户端证书配对使用
	GRPCClientKey string

	// MySQL 配置（数据集存储）
	MySQLHost     string
	MySQLPort     int
	MySQLRootUser string
	MySQLRootPass string
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

// Validate 检查配置值的合法性，返回所有校验错误的聚合。
// 建议在 Load 之后、启动服务之前调用。
func (c *Config) Validate() error {
	var errs []error

	if len(c.JWTSecret) < 16 {
		errs = append(errs, errors.New("HUB_JWT_SECRET must be >= 16 bytes"))
	}
	if c.HTTPAddr != "" && !strings.Contains(c.HTTPAddr, ":") {
		errs = append(errs, errors.New("HUB_HTTP_ADDR must include port (e.g. :8080)"))
	}
	if c.GRPCAddr != "" && !strings.Contains(c.GRPCAddr, ":") {
		errs = append(errs, errors.New("HUB_GRPC_ADDR must include port (e.g. :9090)"))
	}
	if c.RedisAddr != "" && !strings.Contains(c.RedisAddr, ":") {
		errs = append(errs, errors.New("HUB_REDIS_ADDR must include port (e.g. localhost:6379)"))
	}
	if c.QueryTimeout <= 0 {
		errs = append(errs, errors.New("HUB_QUERY_TIMEOUT must be > 0"))
	}
	if c.QueryMaxRows <= 0 {
		errs = append(errs, errors.New("HUB_QUERY_MAX_ROWS must be > 0"))
	}
	if c.DBEncryptionKey == "" {
		errs = append(errs, errors.New("HUB_DB_ENCRYPTION_KEY is required"))
	}
	if c.SchemaCacheTTL < 0 {
		errs = append(errs, errors.New("HUB_SCHEMA_CACHE_TTL must be >= 0"))
	}
	if c.SchemaMaxTables <= 0 {
		errs = append(errs, errors.New("HUB_SCHEMA_MAX_TABLES must be > 0"))
	}
	if c.SchemaMaxColumnsPerTable <= 0 {
		errs = append(errs, errors.New("HUB_SCHEMA_MAX_COLUMNS_PER_TABLE must be > 0"))
	}
	if c.Env != "development" && c.Env != "staging" && c.Env != "production" {
		errs = append(errs, errors.New("HUB_ENV must be one of: development, staging, production"))
	}
	if c.LLMAPIKey != "" && len(c.LLMAPIKey) < 8 {
		errs = append(errs, errors.New("HUB_LLM_API_KEY looks too short (< 8 chars)"))
	}

	return errors.Join(errs...)
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
		MySQLHost:                getenv("HUB_MYSQL_HOST", "localhost"),
		MySQLPort:                int(mustInt32("HUB_MYSQL_PORT", 3306)),
		MySQLRootUser:            getenv("HUB_MYSQL_ROOT_USER", "root"),
		MySQLRootPass:            os.Getenv("HUB_MYSQL_ROOT_PASSWORD"),
	}
	return c, nil
}
