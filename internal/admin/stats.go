package admin

import (
	"net/http"
	"time"
)

func defaultGranularity(d time.Duration) string {
	if d <= 48*time.Hour {
		return "hour"
	}
	return "day"
}

func (h *Handler) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	sinceStr := r.URL.Query().Get("since")
	d := 24 * time.Hour
	if sinceStr != "" {
		var err error
		d, err = parseDurationParam(sinceStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid since: "+err.Error())
			return
		}
	}
	t0 := time.Now().Add(-d)
	stats := h.statsAcc.DashboardStats(t0)
	resp := map[string]interface{}{
		"total_requests": stats.TotalRequests,
		"avg_latency_ms": stats.AvgLatencyMs,
		"error_rate":     stats.ErrorRate,
		"by_model":       stats.ByModel,
		"by_provider":    stats.ByProvider,
	}
	if h.statsAcc.Backfilling() {
		resp["backfilling"] = true
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleGetTimeSeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	d := 24 * time.Hour
	if s := q.Get("since"); s != "" {
		var err error
		d, err = parseDurationParam(s)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid since: "+err.Error())
			return
		}
	}
	granularity := q.Get("granularity")
	if granularity == "" {
		granularity = defaultGranularity(d)
	}
	if granularity != "hour" && granularity != "day" {
		respondError(w, http.StatusBadRequest, "granularity must be hour or day")
		return
	}
	now := time.Now().UTC()
	since := now.Add(-d)
	points := h.statsAcc.TimeSeries(since, now, granularity)
	resp := map[string]interface{}{"points": points}
	if h.statsAcc.Backfilling() {
		resp["backfilling"] = true
	}
	respondJSON(w, http.StatusOK, resp)
}
