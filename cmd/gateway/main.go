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
	"github.com/llmate/gateway/internal/health"
	"github.com/llmate/gateway/internal/logretention"
	"github.com/llmate/gateway/internal/middleware"
	"github.com/llmate/gateway/internal/models"
	"github.com/llmate/gateway/internal/pricing"
	"github.com/llmate/gateway/internal/proxy"
)

// MetricsCollector buffers request logs and persists them asynchronously
// so that the proxy hot path is never blocked on database writes.
type MetricsCollector struct {
	store db.Store
	ch    chan *models.RequestLog
	done  chan struct{}
}

func NewMetricsCollector(store db.Store, bufferSize int) *MetricsCollector {
	return &MetricsCollector{
		store: store,
		ch:    make(chan *models.RequestLog, bufferSize),
		done:  make(chan struct{}),
	}
}

// Record enqueues a log entry. Non-blocking: drops the entry if the buffer is full.
func (m *MetricsCollector) Record(log *models.RequestLog) {
	select {
	case m.ch <- log:
	default:
		slog.Default().Debug("metrics buffer full, dropping log entry", "path", log.Path)
	}
}

// Start launches the background worker that drains the channel and persists logs.
// The worker exits when ctx is cancelled (draining remaining buffered items first),
// or when Close() closes the channel.
func (m *MetricsCollector) Start(ctx context.Context) {
	go func() {
		defer close(m.done)
		for {
			select {
			case log, ok := <-m.ch:
				if !ok {
					return
				}
				m.insert(log)
			case <-ctx.Done():
				// Drain all buffered items before exiting.
				for {
					select {
					case log, ok := <-m.ch:
						if !ok {
							return
						}
						m.insert(log)
					default:
						return
					}
				}
			}
		}
	}()
}

func (m *MetricsCollector) persist(log *models.RequestLog) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if log.ProviderID != "" && log.ResolvedModel != "" && log.EstimatedCostUSD == nil {
		if pm, err := m.store.GetProviderModelCosts(ctx, log.ProviderID, log.ResolvedModel); err == nil && pm != nil {
			b := pricing.ForRequestLog(log, pm)
			if b.TotalUSD > 0 {
				t := b.TotalUSD
				log.EstimatedCostUSD = &t
			}
		}
	}

	if err := m.store.InsertRequestLog(ctx, log); err != nil {
		return err
	}
	return nil
}

func (m *MetricsCollector) insert(log *models.RequestLog) {
	if err := m.persist(log); err != nil {
		slog.Default().Warn("failed to persist request log", "error", err)
	}
}

// PersistSync writes the request log immediately (cost enrichment + insert).
// Used when child rows (e.g. streaming_logs) must exist after the parent request_logs row.
func (m *MetricsCollector) PersistSync(log *models.RequestLog) error {
	return m.persist(log)
}

// Close signals the worker to stop by closing the channel and waits for it to drain.
func (m *MetricsCollector) Close() {
	close(m.ch)
	<-m.done
}

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
	// 1. Load config — fail fast on error.
	cfg, err := config.Load()
	if err != nil {
		slog.Default().Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// 2. Build structured logger.
	logger := buildLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("starting llmate gateway",
		"port", cfg.Port,
		"db_driver", cfg.DBDriver,
		"db", cfg.DBPath,
		"log_level", cfg.LogLevel,
		"health_interval", cfg.HealthInterval,
	)

	// 3. Open database — fail fast on error.
	store, err := db.NewStore(cfg.DBDriver, cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	logger.Info("database ready", "path", cfg.DBPath)

	// 4. Smart router (implements proxy.Router and health.CircuitBreakerReporter).
	smartRouter := proxy.NewSmartRouter(store)

	// 5. Metrics collector (1024-entry buffer).
	metricsCollector := NewMetricsCollector(store, 1024)

	// 6. HTTP client with connection-pool settings but no global timeout
	//    (individual request contexts control per-call deadlines for long LLM calls).
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// 7. Handlers.
	proxyHandler := proxy.NewHandler(smartRouter, metricsCollector, store, httpClient)
	adminHandler := admin.NewHandler(store)
	onboardHandler := admin.NewOnboardHandler(store, httpClient)

	// 8. Health checker.
	healthChecker := health.NewChecker(store, smartRouter, httpClient, cfg.HealthInterval, logger)

	// 9. Chi router.
	r := chi.NewRouter()

	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Logging(logger))
	r.Use(auth.CORSMiddleware())

	// OpenAI-compatible proxy routes (no ACCESS_KEY required).
	r.Post("/v1/chat/completions", proxyHandler.HandleChatCompletions)
	r.Post("/v1/completions", proxyHandler.HandleCompletions)
	r.Post("/v1/embeddings", proxyHandler.HandleEmbeddings)
	r.Post("/v1/images/generations", proxyHandler.HandleImageGenerations)
	r.Post("/v1/audio/speech", proxyHandler.HandleAudioSpeech)
	r.Post("/v1/audio/transcriptions", proxyHandler.HandleAudioTranscriptions)
	r.Get("/v1/models", proxyHandler.HandleListModels)
	r.Get("/v1/models/{model}", proxyHandler.HandleGetModel)

	// Admin routes (ACCESS_KEY required).
	r.Route("/admin", func(r chi.Router) {
		r.Use(auth.AccessKeyMiddleware(cfg.AccessKey))
		// Onboarding routes must be registered before Mount to avoid being shadowed.
		r.Post("/providers/{id}/discover", onboardHandler.HandleDiscover)
		r.Post("/providers/{id}/confirm", onboardHandler.HandleConfirm)
		r.Mount("/", adminHandler.Routes())
	})

	// Alias routes without the /v1 prefix for clients that append paths directly
	// to the base URL (e.g. base=http://localhost:8080 → /chat/completions).
	r.Post("/chat/completions", proxyHandler.HandleChatCompletions)
	r.Post("/completions", proxyHandler.HandleCompletions)
	r.Post("/embeddings", proxyHandler.HandleEmbeddings)
	r.Post("/images/generations", proxyHandler.HandleImageGenerations)
	r.Post("/audio/speech", proxyHandler.HandleAudioSpeech)
	r.Post("/audio/transcriptions", proxyHandler.HandleAudioTranscriptions)
	r.Get("/models", proxyHandler.HandleListModels)
	r.Get("/models/{model}", proxyHandler.HandleGetModel)

	// Frontend static files — catch-all, registered last.
	r.Handle("/*", frontendHandler())

	// 10. HTTP server.
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// 11. Signal handling.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 13. Start metrics worker before accepting traffic.
	metricsCollector.Start(ctx)

	// 14. Start health checker (spawns its own goroutine).
	healthChecker.Start(ctx)

	// 14b. Log body retention: streaming chunks, request bodies, response bodies (daily + on config save).
	logretention.NewWorker(store, logger).Start(ctx)

	// 15. Start listening in background.
	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// 16. Block until signal.
	<-ctx.Done()
	logger.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Graceful shutdown sequence.
	// 1. Shutdown HTTP server first so in-flight handlers finish; they may still call
	//    metricsCollector.Record (sending on a closed channel panics).
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	// 2. Close metrics channel and wait for the worker to drain and exit.
	metricsCollector.Close()

	// 3. Close database.
	if err := store.Close(); err != nil {
		logger.Error("store close error", "error", err)
	}

	logger.Info("shutdown complete")
}
