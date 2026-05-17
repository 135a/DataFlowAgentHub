package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/dataflowagenthub/hub/internal/async"
	"github.com/dataflowagenthub/hub/internal/config"
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
		_ = zl.Sync()
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
	// 如果配置了全局 API 密钥，确保 API 用户存在
	if cfg.GlobalAPIKey != "" {
		if err := seed.EnsureServiceAPIUser(ctx, pool); err != nil {
			zl.Fatal("seed api user", zap.Error(err))
		}
	}

	// 创建 Redis 客户端
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	// 确保在程序退出前关闭 Redis 连接
	defer func() { _ = rdb.Close() }()

	// 建立 NL2SQL 连接
	conn, nl, err := worker.DialNL2SQL(cfg.NL2SQLTarget)
	if err != nil {
		zl.Fatal("nl2sql dial", zap.Error(err))
	}
	// 确保在程序退出前关闭 NL2SQL 连接
	defer func() { _ = conn.Close() }()

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
		Bus:        ssebus.New(),
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
		zl.Info("listening", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zl.Fatal("listen", zap.Error(err))
		}
	}()

	// 设置信号处理，用于优雅关闭
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	// 创建带有超时的关闭上下文
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	// 关闭 OpenTelemetry
	otelCtx, otelCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer otelCancel()
	_ = otelShutdown(otelCtx)
}
