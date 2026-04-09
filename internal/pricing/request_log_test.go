package pricing

import (
	"testing"

	"github.com/llmate/gateway/internal/models"
)

func TestForRequestLog(t *testing.T) {
	in := 1.0
	out := 2.0
	cache := 0.5
	pt := 1000
	ct := 500
	cached := 200

	t.Run("full breakdown with cache", func(t *testing.T) {
		log := &models.RequestLog{
			PromptTokens:     &pt,
			CompletionTokens: &ct,
			CachedTokens:     &cached,
		}
		pm := &models.ProviderModel{
			CostPerMillionInput:     &in,
			CostPerMillionOutput:    &out,
			CostPerMillionCacheRead: &cache,
		}
		b := ForRequestLog(log, pm)
		// (1000-200)/1e6 * 1 = 0.0008
		if g, w := b.InputUSD, (800.0 / 1e6 * in); g != w {
			t.Errorf("InputUSD: got %v want %v", g, w)
		}
		// 500/1e6 * 2 = 0.001
		if g, w := b.OutputUSD, (500.0 / 1e6 * out); g != w {
			t.Errorf("OutputUSD: got %v want %v", g, w)
		}
		// 200/1e6 * 0.5 = 0.0001
		if g, w := b.CachedReadUSD, (200.0 / 1e6 * cache); g != w {
			t.Errorf("CachedReadUSD: got %v want %v", g, w)
		}
		if g, w := b.TotalUSD, b.InputUSD+b.OutputUSD+b.CachedReadUSD; g != w {
			t.Errorf("TotalUSD: got %v want %v", g, w)
		}
	})

	t.Run("nil log or model", func(t *testing.T) {
		if b := ForRequestLog(nil, &models.ProviderModel{}); b.TotalUSD != 0 {
			t.Errorf("nil log: got %v", b.TotalUSD)
		}
		log := &models.RequestLog{PromptTokens: &pt}
		if b := ForRequestLog(log, nil); b.TotalUSD != 0 {
			t.Errorf("nil pm: got %v", b.TotalUSD)
		}
	})

	t.Run("no cached tokens", func(t *testing.T) {
		log := &models.RequestLog{PromptTokens: &pt, CompletionTokens: &ct}
		pm := &models.ProviderModel{CostPerMillionInput: &in, CostPerMillionOutput: &out}
		b := ForRequestLog(log, pm)
		if b.CachedReadUSD != 0 {
			t.Errorf("CachedReadUSD: got %v want 0", b.CachedReadUSD)
		}
		if b.InputUSD != 1000.0/1e6*in {
			t.Errorf("InputUSD: got %v", b.InputUSD)
		}
	})
}
