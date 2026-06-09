package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
	"github.com/llmate/gateway/internal/pricing"
	"github.com/llmate/gateway/internal/proxy"
	"github.com/llmate/gateway/internal/stats"
)

type persistJob struct {
	log           *models.RequestLog
	chunks        []proxy.StreamingLogChunk
	prefixDropped bool
}

type Collector struct {
	store   db.Store
	catalog *proxy.RoutingCatalog
	stats   *stats.Accumulator
	ch      chan persistJob
	done    chan struct{}
}

func NewCollector(store db.Store, catalog *proxy.RoutingCatalog, statsAcc *stats.Accumulator, bufferSize int) *Collector {
	return &Collector{
		store: store, catalog: catalog, stats: statsAcc,
		ch: make(chan persistJob, bufferSize), done: make(chan struct{}),
	}
}

func (m *Collector) Record(log *models.RequestLog) { m.enqueue(persistJob{log: log}) }

func (m *Collector) RecordStreaming(log *models.RequestLog, chunks []proxy.StreamingLogChunk, prefixDropped bool) {
	m.enqueue(persistJob{log: log, chunks: chunks, prefixDropped: prefixDropped})
}

func (m *Collector) enqueue(job persistJob) {
	select {
	case m.ch <- job:
	default:
		slog.Default().Debug("metrics buffer full, dropping log entry", "path", job.log.Path)
	}
}

func (m *Collector) Start(ctx context.Context) {
	go func() {
		defer close(m.done)
		for {
			select {
			case job, ok := <-m.ch:
				if !ok { return }
				m.process(job)
			case <-ctx.Done():
				for {
					select {
					case job, ok := <-m.ch:
						if !ok { return }
						m.process(job)
					default:
						return
					}
				}
			}
		}
	}()
}

func (m *Collector) process(job persistJob) {
	if err := m.persist(job); err != nil {
		slog.Default().Warn("failed to persist request log", "error", err)
	}
}

func (m *Collector) persist(job persistJob) error {
	log := job.log
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if log.ProviderID != "" && log.ResolvedModel != "" && log.EstimatedCostUSD == nil && m.catalog != nil {
		if pm := m.catalog.ProviderModel(log.ProviderID, log.ResolvedModel); pm != nil {
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
	if m.stats != nil {
		var pm *models.ProviderModel
		if m.catalog != nil {
			pm = m.catalog.ProviderModel(log.ProviderID, log.ResolvedModel)
		}
		m.stats.Record(log, pm)
	}
	for i, ch := range job.chunks {
		sl := &models.StreamingLog{
			RequestLogID: log.ID, ChunkIndex: i, Data: ch.Raw, ContentDelta: ch.Delta,
			IsTruncated: job.prefixDropped && i == 0, Timestamp: time.Now().UTC(),
		}
		if err := m.store.InsertStreamingLog(ctx, sl); err != nil {
			return err
		}
	}
	return nil
}

func (m *Collector) Close() { close(m.ch); <-m.done }
