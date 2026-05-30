package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/llmate/gateway/internal/admin"
	"github.com/llmate/gateway/internal/auth"
	"github.com/llmate/gateway/internal/config"
	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/httpx"
	"github.com/llmate/gateway/internal/health"
	"github.com/llmate/gateway/internal/logretention"
	"github.com/llmate/gateway/internal/metrics"
	"github.com/llmate/gateway/internal/middleware"
	"github.com/llmate/gateway/internal/models"
	"github.com/llmate/gateway/internal/proxy"
	"github.com/llmate/gateway/internal/stats"
)

func buildLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(handler)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Default().Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := buildLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("starting llmate gateway",
		"port", cfg.Port,
		"db_driver", cfg.DBDriver,
		"db", cfg.DBPath,
		"log_level", cfg.LogLevel,
		"health_interval", cfg.HealthInterval,
	)

	store, err := db.NewStore(cfg.DBDriver, cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	logger.Info("database ready", "path", cfg.DBPath)

	bootCtx, bootCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer bootCancel()

	configSnap := proxy.NewConfigSnapshot(store)
	if err := configSnap.Reload(bootCtx); err != nil {
		logger.Warn("failed to load config snapshot; using defaults", "error", err)
	}

	bootCfg := configSnap.Get()
	idleConnSec := models.HTTPIdleConnTimeoutSecondsFromConfig(bootCfg)
	outboundPool := httpx.NewPooledClient(time.Duration(idleConnSec) * time.Second)
	httpClient := outboundPool.Client()
	logger.Info("outbound HTTP idle connection timeout", "seconds", idleConnSec)

	routingCatalog := proxy.NewRoutingCatalog(store)
	if err := routingCatalog.Reload(bootCtx); err != nil {
		logger.Error("failed to load routing catalog", "error", err)
		os.Exit(1)
	}

	smartRouter := proxy.NewSmartRouter(routingCatalog)
	statsAcc := stats.NewAccumulator()
	metricsCollector := metrics.NewCollector(store, routingCatalog, statsAcc, 1024)

	reloadRouting := proxy.RoutingChangeNotifier(routingCatalog.ReloadAsync)
	reloadConfig := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := configSnap.Reload(ctx); err != nil {
			logger.Warn("failed to reload config snapshot", "error", err)
		}
	}

	queryWorker := admin.NewQueryWorker(store, 32)
	proxyHandler := proxy.NewHandler(smartRouter, metricsCollector, routingCatalog, configSnap, httpClient)
	adminHandler := admin.NewHandler(store, admin.HandlerConfig{
		OnHTTPIdleConnTimeoutSaved: func(sec int) {
			outboundPool.ApplyIdleConnTimeout(time.Duration(sec) * time.Second)
			logger.Info("outbound HTTP idle connection timeout updated", "seconds", sec)
		},
		OnRoutingChanged: reloadRouting,
		OnConfigChanged:  reloadConfig,
	}, statsAcc, queryWorker)
	onboardHandler := admin.NewOnboardHandler(store, httpClient, reloadRouting)
	healthChecker := health.NewChecker(store, smartRouter, httpClient, cfg.HealthInterval, logger, reloadRouting)

	r := chi.NewRouter()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Logging(logger))
	r.Use(auth.CORSMiddleware())

	r.Post("/v1/chat/completions", proxyHandler.HandleChatCompletions)
	r.Post("/v1/completions", proxyHandler.HandleCompletions)
	r.Post("/v1/embeddings", proxyHandler.HandleEmbeddings)
	r.Post("/v1/images/generations", proxyHandler.HandleImageGenerations)
	r.Post("/v1/audio/speech", proxyHandler.HandleAudioSpeech)
	r.Post("/v1/audio/transcriptions", proxyHandler.HandleAudioTranscriptions)
	r.Get("/v1/models", proxyHandler.HandleListModels)
	r.Get("/v1/models/{model}", proxyHandler.HandleGetModel)

	r.Route("/admin", func(r chi.Router) {
		r.Use(auth.AccessKeyMiddleware(cfg.AccessKey))
		r.Post("/providers/{id}/discover", onboardHandler.HandleDiscover)
		r.Post("/providers/{id}/confirm", onboardHandler.HandleConfirm)
		r.Mount("/", adminHandler.Routes())
	})

	r.Post("/chat/completions", proxyHandler.HandleChatCompletions)
	r.Post("/completions", proxyHandler.HandleCompletions)
	r.Post("/embeddings", proxyHandler.HandleEmbeddings)
	r.Post("/images/generations", proxyHandler.HandleImageGenerations)
	r.Post("/audio/speech", proxyHandler.HandleAudioSpeech)
	r.Post("/audio/transcriptions", proxyHandler.HandleAudioTranscriptions)
	r.Get("/models", proxyHandler.HandleListModels)
	r.Get("/models/{model}", proxyHandler.HandleGetModel)

	r.Handle("/*", frontendHandler())

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	routingCatalog.Start(ctx)
	metricsCollector.Start(ctx)
	queryWorker.Start(ctx)
	go func() {
		bfCtx, bfCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer bfCancel()
		if err := statsAcc.Backfill(bfCtx, store, routingCatalog.ProviderModel); err != nil {
			logger.Warn("stats backfill failed", "error", err)
		} else {
			logger.Info("stats backfill complete")
		}
	}()
	healthChecker.Start(ctx)
	logretention.NewWorker(store, logger).Start(ctx)

	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
	metricsCollector.Close()
	if err := store.Close(); err != nil {
		logger.Error("store close error", "error", err)
	}
	logger.Info("shutdown complete")
}
