package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/dataflowagenthub/hub/internal/async"
	"github.com/dataflowagenthub/hub/internal/config"
	nlv1 "github.com/dataflowagenthub/hub/internal/gen/nl2sql/v1"
	"github.com/dataflowagenthub/hub/internal/grpcserver"
	"github.com/dataflowagenthub/hub/internal/handlers"
	"github.com/dataflowagenthub/hub/internal/llm"
	"github.com/dataflowagenthub/hub/internal/migrate"
	"github.com/dataflowagenthub/hub/internal/nl2sqlexec"
	"github.com/dataflowagenthub/hub/internal/otelsetup"
	"github.com/dataflowagenthub/hub/internal/seed"
	"github.com/dataflowagenthub/hub/internal/ssebus"
	"github.com/dataflowagenthub/hub/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 主函数，应用程序的入口点
func main() {
	// 初始化生产环境的 Zap 日志记录器
	zl, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	// 确保在程序退出前刷新日志缓冲区
	defer func() {
		if err := zl.Sync(); err != nil {
			// logger is shutting down, fall back to stderr
			log.Printf("zap sync: %v", err)
		}
	}()

	// 加载应用程序配置
	cfg, err := config.Load()
	if err != nil {
		zl.Fatal("config", zap.Error(err))
	}

	// 创建上下文
	ctx := context.Background()
	// 创建 PostgreSQL 连接池
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		zl.Fatal("pg connect", zap.Error(err))
	}
	// 确保在程序退出前关闭数据库连接池
	defer pool.Close()

	// 执行数据库迁移
	if err := migrate.Up(ctx, pool); err != nil {
		zl.Fatal("migrate", zap.Error(err))
	}
	// 确保管理员用户存在
	if err := seed.EnsureAdminUser(ctx, pool, cfg); err != nil {
		zl.Fatal("seed admin", zap.Error(err))
	}
	// 创建 Redis 客户端
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	// 确保在程序退出前关闭 Redis 连接
	defer func() {
		if err := rdb.Close(); err != nil {
			zl.Warn("redis close", zap.Error(err))
		}
	}()

	// 建立 NL2SQL 连接（mTLS，若未配置证书则回退到 insecure）
	conn, nl, err := worker.DialNL2SQL(worker.DialOpts{
		Addr:       cfg.NL2SQLTarget,
		ClientCert: cfg.GRPCClientCert,
		ClientKey:  cfg.GRPCClientKey,
		CACert:     cfg.GRPCCACert,
	})
	if err != nil {
		zl.Fatal("nl2sql dial", zap.Error(err))
	}
	// 确保在程序退出前关闭 NL2SQL 连接
	defer func() {
		if err := conn.Close(); err != nil {
			zl.Warn("grpc conn close", zap.Error(err))
		}
	}()

	// 连接到 NATS 服务器
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		zl.Warn("nats connect failed, async tasks disabled", zap.Error(err))
	}

	// 创建 NL2SQL 执行器
	nl2sqlExec := nl2sqlexec.NewExecutor(nl, cfg.QueryMaxRows, cfg.QueryTimeout)

	// 创建 LLM 客户端，用于 AI 校验
	llmClient := &llm.Client{
		BaseURL: cfg.LLMBaseURL,
		APIKey:  cfg.LLMAPIKey,
	}

	// 初始化应用程序结构体
	app := &handlers.App{
		Cfg:        cfg,
		Log:        zl,
		DB:         pool,
		Redis:      rdb,
		Nl2sql:     nl,
		Bus:        func() *ssebus.Bus { b := ssebus.New(); b.SetLogger(zl); return b }(),
		NATS:       nc,
		AsyncTask:  async.NewClient(pool, nc, zl),
		NL2SQLExec: nl2sqlExec,
		LlmClient:  llmClient,
	}

	// 初始化 OpenTelemetry
	otelShutdown, err := otelsetup.Init()
	if err != nil {
		zl.Fatal("otel", zap.Error(err))
	}

	// 设置路由和处理程序
	base := handlers.Routes(app)
	// 创建带有 OpenTelemetry 支持的 HTTP 处理程序，并过滤掉 /metrics 端点
	h := otelhttp.NewHandler(base, "hub-api",
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/metrics" }))

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 在 goroutine 中启动 HTTP 服务器
	go func() {
		zl.Info("listening", zap.String("http_addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zl.Fatal("http listen", zap.Error(err))
		}
	}()

	// 启动 gRPC 服务端（内部回调）
	grpcOpts := []grpc.ServerOption{}

	// 加载 mTLS 凭证
	if cfg.GRPCServerCert != "" && cfg.GRPCServerKey != "" && cfg.GRPCCACert != "" {
		serverCert, err := tls.LoadX509KeyPair(cfg.GRPCServerCert, cfg.GRPCServerKey)
		if err != nil {
			zl.Fatal("load grpc server cert", zap.Error(err))
		}
		caCert, err := os.ReadFile(cfg.GRPCCACert)
		if err != nil {
			zl.Fatal("read grpc ca cert", zap.Error(err))
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			zl.Fatal("failed to parse CA certificate")
		}

		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
			MinVersion:   tls.VersionTLS12,
		}
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
		zl.Info("grpc mTLS enabled", zap.String("ca_cert", cfg.GRPCCACert))
	} else {
		zl.Warn("grpc mTLS disabled (no cert paths configured), using insecure")
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	nlv1.RegisterHubInternalServiceServer(grpcServer, grpcserver.NewInternalServer(app))

	grpcLis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		zl.Fatal("grpc listen", zap.Error(err))
	}
	go func() {
		zl.Info("grpc listening", zap.String("grpc_addr", cfg.GRPCAddr))
		if err := grpcServer.Serve(grpcLis); err != nil {
			zl.Fatal("grpc serve", zap.Error(err))
		}
	}()

	// 设置信号处理，用于优雅关闭
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	zl.Info("shutting down...")

	// 先停 gRPC 服务端（不再接受新请求，等待正在处理的完成）
	grpcServer.GracefulStop()

	// 再停 HTTP 服务端
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		zl.Warn("http server shutdown", zap.Error(err))
	}
	// 关闭 OpenTelemetry
	otelCtx, otelCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer otelCancel()
	if err := otelShutdown(otelCtx); err != nil {
		zl.Warn("otel shutdown", zap.Error(err))
	}
}
