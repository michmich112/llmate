package proxy

import (
	"strings"
	"testing"
)

func TestRewriteSSEDataLineForClientModel(t *testing.T) {
	line := `data: {"id":"1","model":"backend","choices":[]}`
	got := rewriteSSEDataLineForClientModel(line, "alias-name")
	if !strings.Contains(got, `"model":"alias-name"`) {
		t.Fatalf("want rewritten model, got %q", got)
	}
	if strings.Contains(got, `"model":"backend"`) {
		t.Fatalf("should not keep backend model: %q", got)
	}
	if strings.HasPrefix(got, "data:") == false {
		t.Fatalf("expected data: prefix: %q", got)
	}
	// [DONE] unchanged
	done := "data: [DONE]"
	if rewriteSSEDataLineForClientModel(done, "x") != done {
		t.Errorf("[DONE] line should be unchanged")
	}
	// Non-JSON payload unchanged
	if g := rewriteSSEDataLineForClientModel("data: hello", "x"); g != "data: hello" {
		t.Errorf("non-json data line unchanged, got %q", g)
	}
	noModel := `data: {"id":"1"}`
	if gotNo := rewriteSSEDataLineForClientModel(noModel, "x"); gotNo != noModel {
		t.Errorf("line without model should be unchanged: got %q", gotNo)
	}
}

func TestMergeStreamUsage(t *testing.T) {
	t.Run("preserves cached when later chunk omits details", func(t *testing.T) {
		cached200 := 200
		u := mergeStreamUsage(nil, 10, 1, 11, &cached200)
		if !u.CachedTokensReported || u.CachedTokens != 200 {
			t.Fatalf("after first chunk: %+v", u)
		}
		u = mergeStreamUsage(u, 1000, 500, 1500, nil)
		if u.PromptTokens != 1000 || u.CompletionTokens != 500 || u.TotalTokens != 1500 {
			t.Fatalf("tokens: %+v", u)
		}
		if u.CachedTokens != 200 || !u.CachedTokensReported {
			t.Fatalf("cached should be preserved: %+v", u)
		}
	})

	t.Run("updates cached when details reappear", func(t *testing.T) {
		c1 := 100
		u := mergeStreamUsage(nil, 50, 0, 50, &c1)
		c2 := 0
		u = mergeStreamUsage(u, 50, 0, 50, &c2)
		if u.CachedTokens != 0 || !u.CachedTokensReported {
			t.Fatalf("want cached 0 reported: %+v", u)
		}
	})
}
