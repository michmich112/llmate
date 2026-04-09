package logretention

import (
	"testing"

	"github.com/llmate/gateway/internal/models"
)

func TestStreamingRetentionDaysFromConfig(t *testing.T) {
	d, ok := StreamingRetentionDaysFromConfig(map[string]string{})
	if !ok || d != models.DefaultStreamingLogBodyRetentionDays {
		t.Fatalf("empty config: got %d ok=%v", d, ok)
	}
	d, ok = StreamingRetentionDaysFromConfig(map[string]string{"streaming_log_body_retention_days": "14"})
	if !ok || d != 14 {
		t.Fatalf("14: got %d ok=%v", d, ok)
	}
	_, ok = StreamingRetentionDaysFromConfig(map[string]string{"streaming_log_body_retention_days": "0"})
	if ok {
		t.Fatal("0 should be invalid")
	}
	_, ok = StreamingRetentionDaysFromConfig(map[string]string{"streaming_log_body_retention_days": "99999"})
	if ok {
		t.Fatal("above max should be invalid")
	}
	_, ok = StreamingRetentionDaysFromConfig(map[string]string{"streaming_log_body_retention_days": "x"})
	if ok {
		t.Fatal("non-numeric should be invalid")
	}
}

func TestRequestLogBodyRetentionDaysFromConfig(t *testing.T) {
	d, ok := RequestLogBodyRetentionDaysFromConfig(map[string]string{})
	if !ok || d != models.DefaultRequestLogBodyRetentionDays {
		t.Fatalf("empty config: got %d ok=%v", d, ok)
	}
	d, ok = RequestLogBodyRetentionDaysFromConfig(map[string]string{"request_log_body_retention_days": "7"})
	if !ok || d != 7 {
		t.Fatalf("7: got %d ok=%v", d, ok)
	}
	_, ok = RequestLogBodyRetentionDaysFromConfig(map[string]string{"request_log_body_retention_days": "0"})
	if ok {
		t.Fatal("0 should be invalid")
	}
}

func TestResponseLogBodyRetentionDaysFromConfig(t *testing.T) {
	d, ok := ResponseLogBodyRetentionDaysFromConfig(map[string]string{})
	if !ok || d != models.DefaultResponseLogBodyRetentionDays {
		t.Fatalf("empty config: got %d ok=%v", d, ok)
	}
	d, ok = ResponseLogBodyRetentionDaysFromConfig(map[string]string{"response_log_body_retention_days": "90"})
	if !ok || d != 90 {
		t.Fatalf("90: got %d ok=%v", d, ok)
	}
}
