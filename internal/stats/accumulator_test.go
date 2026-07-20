package stats

import (
	"testing"
	"time"

	"github.com/llmate/gateway/internal/models"
)

func TestAccumulator_ByModelUsesResolvedModel(t *testing.T) {
	acc := NewAccumulator()
	now := time.Now().UTC()
	tokens := 10

	acc.Record(&models.RequestLog{
		Timestamp: now, StatusCode: 200, TotalTimeMs: 5,
		RequestedModel: "fast", ResolvedModel: "llama3",
		TotalTokens: &tokens,
	}, nil)
	acc.Record(&models.RequestLog{
		Timestamp: now, StatusCode: 200, TotalTimeMs: 7,
		RequestedModel: "fast", ResolvedModel: "llama3",
		TotalTokens: &tokens,
	}, nil)
	acc.Record(&models.RequestLog{
		Timestamp: now, StatusCode: 200, TotalTimeMs: 9,
		RequestedModel: "llama3", ResolvedModel: "llama3",
		TotalTokens: &tokens,
	}, nil)

	stats := acc.DashboardStats(now.Add(-time.Hour))
	if len(stats.ByModel) != 1 {
		t.Fatalf("ByModel length: got %d, want 1", len(stats.ByModel))
	}
	if stats.ByModel[0].Model != "llama3" {
		t.Errorf("ByModel[0].Model: got %q, want %q", stats.ByModel[0].Model, "llama3")
	}
	if stats.ByModel[0].RequestCount != 3 {
		t.Errorf("ByModel[0].RequestCount: got %d, want 3", stats.ByModel[0].RequestCount)
	}
}

func TestAccumulator_ByModelFallsBackToRequested(t *testing.T) {
	acc := NewAccumulator()
	now := time.Now().UTC()
	acc.Record(&models.RequestLog{
		Timestamp: now, StatusCode: 503, TotalTimeMs: 1,
		RequestedModel: "fast",
	}, nil)

	stats := acc.DashboardStats(now.Add(-time.Hour))
	if len(stats.ByModel) != 1 || stats.ByModel[0].Model != "fast" {
		t.Fatalf("expected fallback to requested model, got %+v", stats.ByModel)
	}
}
