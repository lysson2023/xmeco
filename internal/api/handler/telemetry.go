package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"xmeco/internal/repository/postgres"
)

type TelemetryHandler struct{ repo *postgres.TelemetryRepo }

func NewTelemetryHandler(repo *postgres.TelemetryRepo) *TelemetryHandler { return &TelemetryHandler{repo: repo} }

func (h *TelemetryHandler) Realtime(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	list, err := h.repo.Realtime(r.Context(), deviceID)
	if err != nil {
		serverErr(w, err)
		return
	}
	if list == nil {
		list = []postgres.TelemetryPoint{}
	}
	ok(w, list)
}

func (h *TelemetryHandler) History(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	metric := r.URL.Query().Get("metric")
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours <= 0 {
		hours = 24
	}

	list, err := h.repo.History(r.Context(), deviceID, metric, hours)
	if err != nil {
		serverErr(w, err)
		return
	}
	if list == nil {
		list = []postgres.HistoryPoint{}
	}
	ok(w, list)
}

func (h *TelemetryHandler) Stats(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	if deviceID > 0 {
		// 单设备统计
		list, err := h.repo.DeviceStats(r.Context(), deviceID)
		if err != nil {
			serverErr(w, err)
			return
		}
		if list == nil {
			list = []postgres.DeviceStat{}
		}
		ok(w, list)
		return
	}
	// 系统级在线/离线统计
	stats, err := h.repo.SystemOnlineStats(r.Context())
	if err != nil {
		slog.Warn("TelemetryHandler.Stats query failed", "err", err)
		ok(w, map[string]any{"online": 0, "offline": 0, "total": 0})
		return
	}
	ok(w, stats)
}