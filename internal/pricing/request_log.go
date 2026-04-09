package pricing

import "github.com/llmate/gateway/internal/models"

// Breakdown is the USD cost split for a single request log using provider model rates.
// All fields are zero when pricing or token data is missing for that component.
type Breakdown struct {
	// InputUSD is cost for non-cached prompt tokens (prompt minus cached), at input rate.
	InputUSD float64
	// OutputUSD is cost for completion tokens at output rate.
	OutputUSD float64
	// CachedReadUSD is cost for cached prompt tokens at cache-read rate.
	CachedReadUSD float64
	// TotalUSD is InputUSD + OutputUSD + CachedReadUSD.
	TotalUSD float64
}

// ForRequestLog estimates cost from stored token counts and per-million rates.
// pm may be nil or have nil rate fields; missing data yields zero for that component.
func ForRequestLog(log *models.RequestLog, pm *models.ProviderModel) Breakdown {
	if log == nil || pm == nil {
		return Breakdown{}
	}
	var b Breakdown
	if log.PromptTokens != nil && pm.CostPerMillionInput != nil {
		billed := *log.PromptTokens
		if log.CachedTokens != nil {
			billed -= *log.CachedTokens
		}
		if billed > 0 {
			b.InputUSD = float64(billed) / 1e6 * *pm.CostPerMillionInput
		}
	}
	if log.CompletionTokens != nil && pm.CostPerMillionOutput != nil {
		b.OutputUSD = float64(*log.CompletionTokens) / 1e6 * *pm.CostPerMillionOutput
	}
	if log.CachedTokens != nil && *log.CachedTokens > 0 && pm.CostPerMillionCacheRead != nil {
		b.CachedReadUSD = float64(*log.CachedTokens) / 1e6 * *pm.CostPerMillionCacheRead
	}
	b.TotalUSD = b.InputUSD + b.OutputUSD + b.CachedReadUSD
	return b
}
