package stats

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/llmate/gateway/internal/db"
	"github.com/llmate/gateway/internal/models"
	"github.com/llmate/gateway/internal/pricing"
)

const retentionDays = 90

type modelKey struct {
	model string
}

type providerKey struct {
	id   string
	name string
}

type bucketStats struct {
	requests         int
	successCount     int
	errorCount       int
	latencySumMs     int64
	inputTokens      int
	promptTokens     int
	completionTokens int
	totalTokens      int
	cachedTokens     int
	totalCostUSD     float64
	inputCostUSD     float64
	outputCostUSD    float64
	cachedCostUSD    float64
	byModel          map[string]*models.ModelStats
	byProvider       map[string]*models.ProviderStats
}

type Accumulator struct {
	mu          sync.RWMutex
	hourly      map[string]*bucketStats
	backfilling atomic.Bool
}

func NewAccumulator() *Accumulator {
	return &Accumulator{hourly: make(map[string]*bucketStats)}
}

func (a *Accumulator) Backfilling() bool { return a.backfilling.Load() }

func (a *Accumulator) Record(log *models.RequestLog, pm *models.ProviderModel) {
	if log == nil {
		return
	}
	b := pricing.ForRequestLog(log, pm)
	key := hourBucketKey(log.Timestamp)
	a.mu.Lock()
	defer a.mu.Unlock()
	bkt := a.ensureBucket(key)
	a.applyLog(bkt, log, b)
	a.pruneLocked(time.Now().UTC().Add(-retentionDays * 24 * time.Hour))
}

func (a *Accumulator) DashboardStats(since time.Time) *models.DashboardStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := &models.DashboardStats{ByModel: []models.ModelStats{}, ByProvider: []models.ProviderStats{}}
	modelAgg := map[string]*models.ModelStats{}
	provAgg := map[string]*models.ProviderStats{}
	var total int
	var latencySum int64
	var errors int
	for key, b := range a.hourly {
		t, err := time.Parse("2006-01-02T15:00:00", key)
		if err != nil || t.Before(since.UTC()) {
			continue
		}
		total += b.requests
		latencySum += b.latencySumMs
		errors += b.errorCount
		for m, ms := range b.byModel {
			if cur, ok := modelAgg[m]; ok {
				cur.RequestCount += ms.RequestCount
				cur.ErrorCount += ms.ErrorCount
				cur.TotalTokens += ms.TotalTokens
				cur.AvgLatencyMs = weightedAvg(cur.AvgLatencyMs, cur.RequestCount-ms.RequestCount, ms.AvgLatencyMs, ms.RequestCount)
			} else {
				copy := *ms
				modelAgg[m] = &copy
			}
		}
		for id, ps := range b.byProvider {
			if cur, ok := provAgg[id]; ok {
				cur.RequestCount += ps.RequestCount
				cur.ErrorCount += ps.ErrorCount
				cur.AvgLatencyMs = weightedAvg(cur.AvgLatencyMs, cur.RequestCount-ps.RequestCount, ps.AvgLatencyMs, ps.RequestCount)
			} else {
				copy := *ps
				provAgg[id] = &copy
			}
		}
	}
	out.TotalRequests = total
	if total > 0 {
		out.AvgLatencyMs = float64(latencySum) / float64(total)
		out.ErrorRate = float64(errors) / float64(total)
	}
	for _, ms := range modelAgg {
		out.ByModel = append(out.ByModel, *ms)
	}
	sort.Slice(out.ByModel, func(i, j int) bool { return out.ByModel[i].RequestCount > out.ByModel[j].RequestCount })
	for _, ps := range provAgg {
		out.ByProvider = append(out.ByProvider, *ps)
	}
	sort.Slice(out.ByProvider, func(i, j int) bool { return out.ByProvider[i].RequestCount > out.ByProvider[j].RequestCount })
	return out
}

func (a *Accumulator) TimeSeries(since, until time.Time, granularity string) []models.TimeSeriesPoint {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var points []models.TimeSeriesPoint
	switch granularity {
	case "hour":
		for t := since.UTC().Truncate(time.Hour); !t.After(until.UTC()); t = t.Add(time.Hour) {
			key := t.Format("2006-01-02T15:00:00")
			points = append(points, a.pointFromBucket(key))
		}
	case "day":
		for t := since.UTC().Truncate(24 * time.Hour); !t.After(until.UTC()); t = t.Add(24 * time.Hour) {
			key := t.Format("2006-01-02")
			points = append(points, a.aggregateDay(key))
		}
	default:
		return nil
	}
	return points
}

func (a *Accumulator) Backfill(ctx context.Context, store db.Store, costs func(providerID, modelID string) *models.ProviderModel) error {
	a.backfilling.Store(true)
	defer a.backfilling.Store(false)
	since := time.Now().UTC().Add(-retentionDays * 24 * time.Hour)
	offset := 0
	const page = 500
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		logs, _, err := store.QueryRequestLogs(ctx, models.LogFilter{Since: &since, Limit: page, Offset: offset})
		if err != nil {
			return err
		}
		if len(logs) == 0 {
			return nil
		}
		for i := range logs {
			var pm *models.ProviderModel
			if costs != nil {
				pm = costs(logs[i].ProviderID, logs[i].ResolvedModel)
			}
			a.Record(&logs[i], pm)
		}
		if len(logs) < page {
			return nil
		}
		offset += page
	}
}

func (a *Accumulator) pointFromBucket(key string) models.TimeSeriesPoint {
	b, ok := a.hourly[key]
	if !ok {
		return models.TimeSeriesPoint{Bucket: key}
	}
	return models.TimeSeriesPoint{
		Bucket: key, Requests: b.requests, SuccessCount: b.successCount, ErrorCount: b.errorCount,
		InputTokens: b.inputTokens, PromptTokens: b.promptTokens, CompletionTokens: b.completionTokens,
		TotalTokens: b.totalTokens, CachedTokens: b.cachedTokens, TotalCostUSD: b.totalCostUSD,
		InputCostUSD: b.inputCostUSD, OutputCostUSD: b.outputCostUSD, CachedCostUSD: b.cachedCostUSD,
	}
}

func (a *Accumulator) aggregateDay(dayKey string) models.TimeSeriesPoint {
	p := models.TimeSeriesPoint{Bucket: dayKey}
	for key, b := range a.hourly {
		if len(key) < 10 || key[:10] != dayKey {
			continue
		}
		p.Requests += b.requests
		p.SuccessCount += b.successCount
		p.ErrorCount += b.errorCount
		p.InputTokens += b.inputTokens
		p.PromptTokens += b.promptTokens
		p.CompletionTokens += b.completionTokens
		p.TotalTokens += b.totalTokens
		p.CachedTokens += b.cachedTokens
		p.TotalCostUSD += b.totalCostUSD
		p.InputCostUSD += b.inputCostUSD
		p.OutputCostUSD += b.outputCostUSD
		p.CachedCostUSD += b.cachedCostUSD
	}
	return p
}

func (a *Accumulator) ensureBucket(key string) *bucketStats {
	b, ok := a.hourly[key]
	if !ok {
		b = &bucketStats{byModel: map[string]*models.ModelStats{}, byProvider: map[string]*models.ProviderStats{}}
		a.hourly[key] = b
	}
	return b
}

func (a *Accumulator) applyLog(b *bucketStats, log *models.RequestLog, costs pricing.Breakdown) {
	b.requests++
	b.latencySumMs += int64(log.TotalTimeMs)
	if log.StatusCode >= 200 && log.StatusCode < 300 {
		b.successCount++
	}
	if log.StatusCode >= 400 {
		b.errorCount++
	}
	if log.PromptTokens != nil {
		b.promptTokens += *log.PromptTokens
		cached := 0
		if log.CachedTokens != nil {
			cached = *log.CachedTokens
		}
		b.inputTokens += max(0, *log.PromptTokens-cached)
	}
	if log.CompletionTokens != nil {
		b.completionTokens += *log.CompletionTokens
	}
	if log.TotalTokens != nil {
		b.totalTokens += *log.TotalTokens
	}
	if log.CachedTokens != nil {
		b.cachedTokens += *log.CachedTokens
	}
	b.totalCostUSD += costs.TotalUSD
	b.inputCostUSD += costs.InputUSD
	b.outputCostUSD += costs.OutputUSD
	b.cachedCostUSD += costs.CachedReadUSD

	modelName := log.RequestedModel
	if modelName == "" {
		modelName = log.ResolvedModel
	}
	ms := b.byModel[modelName]
	if ms == nil {
		ms = &models.ModelStats{Model: modelName}
		b.byModel[modelName] = ms
	}
	ms.RequestCount++
	ms.AvgLatencyMs = weightedAvg(ms.AvgLatencyMs, ms.RequestCount-1, float64(log.TotalTimeMs), 1)
	if log.StatusCode >= 400 {
		ms.ErrorCount++
	}
	if log.TotalTokens != nil {
		ms.TotalTokens += *log.TotalTokens
	}

	pid := log.ProviderID
	pname := log.ProviderName
	ps := b.byProvider[pid]
	if ps == nil {
		ps = &models.ProviderStats{ProviderID: pid, ProviderName: pname}
		b.byProvider[pid] = ps
	}
	ps.RequestCount++
	ps.AvgLatencyMs = weightedAvg(ps.AvgLatencyMs, ps.RequestCount-1, float64(log.TotalTimeMs), 1)
	if log.StatusCode >= 400 {
		ps.ErrorCount++
	}
}

func (a *Accumulator) pruneLocked(cutoff time.Time) {
	for key := range a.hourly {
		t, err := time.Parse("2006-01-02T15:00:00", key)
		if err == nil && t.Before(cutoff) {
			delete(a.hourly, key)
		}
	}
}

func hourBucketKey(t time.Time) string {
	return t.UTC().Truncate(time.Hour).Format("2006-01-02T15:00:00")
}

func weightedAvg(a float64, aCount int, b float64, bCount int) float64 {
	total := aCount + bCount
	if total == 0 {
		return 0
	}
	return (a*float64(aCount) + b*float64(bCount)) / float64(total)
}
