package admin

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/llmate/gateway/internal/models"
)

func defaultGranularity(d time.Duration) string {
	if d <= 48*time.Hour {
		return "hour"
	}
	return "day"
}

type statsWindowError string

func (e statsWindowError) Error() string { return string(e) }

const (
	errMissingFromTo statsWindowError = "from and to must both be provided"
	errInvalidFrom   statsWindowError = "invalid from: must be RFC3339"
	errInvalidTo     statsWindowError = "invalid to: must be RFC3339"
	errFromAfterTo   statsWindowError = "from must be before or equal to to"
)

func parseRFC3339Param(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// parseStatsWindow resolves the dashboard time window from query params.
// Absolute from/to (RFC3339) take precedence over relative since durations.
// Returns useDB=true when the caller should query SQLite rather than the in-memory accumulator.
func parseStatsWindow(q url.Values) (since, until time.Time, useDB bool, err error) {
	now := time.Now().UTC()
	fromStr := q.Get("from")
	toStr := q.Get("to")

	if fromStr != "" || toStr != "" {
		if fromStr == "" || toStr == "" {
			return time.Time{}, time.Time{}, false, errMissingFromTo
		}
		since, err = parseRFC3339Param(fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, false, errInvalidFrom
		}
		until, err = parseRFC3339Param(toStr)
		if err != nil {
			return time.Time{}, time.Time{}, false, errInvalidTo
		}
		if until.Before(since) {
			return time.Time{}, time.Time{}, false, errFromAfterTo
		}
		return since.UTC(), until.UTC(), true, nil
	}

	d := 24 * time.Hour
	if s := q.Get("since"); s != "" {
		d, err = parseDurationParam(s)
		if err != nil {
			return time.Time{}, time.Time{}, false, fmt.Errorf("invalid since: %w", err)
		}
	}
	return now.Add(-d), now, false, nil
}

func (h *Handler) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	since, until, useDB, err := parseStatsWindow(r.URL.Query())
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var stats *models.DashboardStats
	if useDB {
		stats, err = h.store.GetDashboardStats(r.Context(), since, until)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to load stats")
			return
		}
	} else {
		stats = h.statsAcc.DashboardStats(since)
	}

	resp := map[string]interface{}{
		"total_requests": stats.TotalRequests,
		"avg_latency_ms": stats.AvgLatencyMs,
		"error_rate":     stats.ErrorRate,
		"by_model":       stats.ByModel,
		"by_provider":    stats.ByProvider,
	}
	if !useDB && h.statsAcc.Backfilling() {
		resp["backfilling"] = true
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleGetTimeSeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	since, until, useDB, err := parseStatsWindow(q)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	granularity := q.Get("granularity")
	if granularity == "" {
		granularity = defaultGranularity(until.Sub(since))
	}
	if granularity != "hour" && granularity != "day" {
		respondError(w, http.StatusBadRequest, "granularity must be hour or day")
		return
	}

	var points []models.TimeSeriesPoint
	if useDB {
		points, err = h.store.GetTimeSeries(r.Context(), since, until, granularity)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to load time series")
			return
		}
	} else {
		points = h.statsAcc.TimeSeries(since, until, granularity)
	}

	resp := map[string]interface{}{"points": points}
	if !useDB && h.statsAcc.Backfilling() {
		resp["backfilling"] = true
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleGetLifetimeCost(w http.ResponseWriter, r *http.Request) {
	cost, err := h.store.GetLifetimeCost(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load lifetime cost")
		return
	}
	respondJSON(w, http.StatusOK, cost)
}
