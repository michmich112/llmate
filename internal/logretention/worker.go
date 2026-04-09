package logretention

import (
	"context"
	"log/slog"
	"time"

	"github.com/llmate/gateway/internal/db"
)

// Worker runs periodic purges for streaming chunk bodies and request log request/response bodies.
type Worker struct {
	store  db.Store
	logger *slog.Logger
}

// NewWorker creates a retention worker. logger must be non-nil.
func NewWorker(store db.Store, logger *slog.Logger) *Worker {
	return &Worker{store: store, logger: logger}
}

// Start runs an immediate purge cycle then one every 24h until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	w.logger.Info("starting log body retention worker")

	go func() {
		defer w.logger.Info("stopping log body retention worker")

		w.runOnce(ctx)

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.runOnce(ctx)
			}
		}
	}()
}

func (w *Worker) runOnce(ctx context.Context) {
	cfg, err := w.store.GetAllConfig(ctx)
	if err != nil {
		w.logger.Warn("log retention: failed to load config", "error", err)
		return
	}

	if days, ok := StreamingRetentionDaysFromConfig(cfg); !ok {
		w.logger.Warn("log retention: invalid streaming_log_body_retention_days, skipping streaming chunk purge")
	} else {
		n, err := PurgeStreamingChunkBodies(ctx, w.store, days)
		if err != nil {
			w.logger.Warn("log retention: streaming chunk purge failed", "error", err)
		} else if n > 0 {
			w.logger.Info("log retention: purged old streaming chunk bodies", "rows", n, "retention_days", days)
		}
	}

	if days, ok := RequestLogBodyRetentionDaysFromConfig(cfg); !ok {
		w.logger.Warn("log retention: invalid request_log_body_retention_days, skipping request body purge")
	} else {
		n, err := PurgeRequestLogRequestBodies(ctx, w.store, days)
		if err != nil {
			w.logger.Warn("log retention: request body purge failed", "error", err)
		} else if n > 0 {
			w.logger.Info("log retention: purged old request bodies", "rows", n, "retention_days", days)
		}
	}

	if days, ok := ResponseLogBodyRetentionDaysFromConfig(cfg); !ok {
		w.logger.Warn("log retention: invalid response_log_body_retention_days, skipping response body purge")
	} else {
		n, err := PurgeRequestLogResponseBodies(ctx, w.store, days)
		if err != nil {
			w.logger.Warn("log retention: response body purge failed", "error", err)
		} else if n > 0 {
			w.logger.Info("log retention: purged old response bodies", "rows", n, "retention_days", days)
		}
	}
}
