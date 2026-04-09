package admin

import (
	"net/http"
	"time"
)

// defaultGranularity returns "hour" for windows up to 48 hours, "day" otherwise.
// This rule is applied when the client does not specify a granularity param.
func defaultGranularity(d time.Duration) string {
	if d <= 48*time.Hour {
		return "hour"
	}
	return "day"
}

// HandleGetStats returns aggregated dashboard statistics for a given time window.
// The "since" query parameter accepts Go durations and the "Nd" day shorthand
// (e.g. "24h", "7d", "30d"). Defaults to the last 24 hours when omitted.
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
	stats, err := h.store.GetDashboardStats(r.Context(), t0)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

// HandleGetTimeSeries returns request metrics bucketed by time.
//
// Query params:
//   - since: duration string (default "24h"); e.g. "24h", "7d", "30d"
//   - granularity: "hour" or "day" (default: "hour" for ≤48h windows, "day" otherwise)
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
	points, err := h.store.GetTimeSeries(r.Context(), since, now, granularity)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get time series")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"points": points})
}
