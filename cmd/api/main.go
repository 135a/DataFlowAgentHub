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

func main() {
	zl, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer func() { _ = zl.Sync() }()

	cfg, err := config.Load()
	if err != nil {
		zl.Fatal("config", zap.Error(err))
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		zl.Fatal("pg connect", zap.Error(err))
	}
	defer pool.Close()

	if err := migrate.Up(ctx, pool); err != nil {
		zl.Fatal("migrate", zap.Error(err))
	}
	if err := seed.EnsureAdminUser(ctx, pool, cfg); err != nil {
		zl.Fatal("seed admin", zap.Error(err))
	}
	if cfg.GlobalAPIKey != "" {
		if err := seed.EnsureServiceAPIUser(ctx, pool); err != nil {
			zl.Fatal("seed api user", zap.Error(err))
		}
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = rdb.Close() }()

	conn, nl, err := worker.DialNL2SQL(cfg.NL2SQLTarget)
	if err != nil {
		zl.Fatal("nl2sql dial", zap.Error(err))
	}
	defer func() { _ = conn.Close() }()

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		zl.Warn("nats connect failed, async tasks disabled", zap.Error(err))
	}

	nl2sqlExec := nl2sqlexec.NewExecutor(nl, cfg.QueryMaxRows, cfg.QueryTimeout)

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
	}

	otelShutdown, err := otelsetup.Init()
	if err != nil {
		zl.Fatal("otel", zap.Error(err))
	}

	base := handlers.Routes(app)
	h := otelhttp.NewHandler(base, "hub-api",
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/metrics" }))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		zl.Info("listening", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zl.Fatal("listen", zap.Error(err))
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	otelCtx, otelCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer otelCancel()
	_ = otelShutdown(otelCtx)
}
