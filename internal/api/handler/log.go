package handler

import (
	"log/slog"
	"net/http"
	"time"

	"xmeco/internal/repository/postgres"
)

type LogHandler struct{ repo *postgres.LogRepo }

func NewLogHandler(repo *postgres.LogRepo) *LogHandler { return &LogHandler{repo: repo} }

// defaultTimeRange 为空的时间范围字段填充默认值。
// - end: 如果为空则设为当前时间；如果仅有日期部分（10 字符）则补全为 23:59:59。
// - start: 如果为空则设为 1 个月前。
func defaultTimeRange(start, end string) (string, string) {
	if end == "" {
		end = time.Now().Format("2006-01-02 15:04:05")
	} else if len(end) == 10 {
		end += " 23:59:59"
	}
	if start == "" {
		start = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	return start, end
}

func (h *LogHandler) Telemetry(w http.ResponseWriter, r *http.Request) {
	filter := postgres.TelemetryFilter{
		DeviceID:   queryInt(r, "device_id"),
		BuildingID: queryInt(r, "building_id"),
		Metric:     r.URL.Query().Get("metric"),
		Interval:   r.URL.Query().Get("interval"),
		Start:      r.URL.Query().Get("start"),
		End:        r.URL.Query().Get("end"),
	}

	filter.Start, filter.End = defaultTimeRange(filter.Start, filter.End)

	// 白名单校验 interval
	if filter.Interval != "" && !postgres.ValidIntervals[filter.Interval] {
		writeJSON(w, http.StatusBadRequest, M{"error": "无效的时间间隔，可选值：minute/hour/day/week/month/year/raw"})
		return
	}

	export := r.URL.Query().Get("export") == "csv"
	if export {
		if err := h.repo.ExportTelemetryCSV(r.Context(), w, filter); err != nil {
			slog.Warn("Telemetry export failed", "err", err)
		}
		return
	}

	list, err := h.repo.Telemetry(r.Context(), filter)
	if err != nil {
		serverErr(w, err)
		return
	}
	if list == nil {
		list = []map[string]any{}
	}
	ok(w, list)
}

func (h *LogHandler) Controls(w http.ResponseWriter, r *http.Request) {
	filter := postgres.ControlFilter{
		DeviceID:   queryInt(r, "device_id"),
		BuildingID: queryInt(r, "building_id"),
		ProjectID:  queryInt(r, "project_id"),
		Start:      r.URL.Query().Get("start"),
		End:        r.URL.Query().Get("end"),
	}

	filter.Start, filter.End = defaultTimeRange(filter.Start, filter.End)

	export := r.URL.Query().Get("export") == "csv"
	if export {
		if err := h.repo.ExportControlsCSV(r.Context(), w, filter); err != nil {
			slog.Warn("Controls export failed", "err", err)
		}
		return
	}

	list, err := h.repo.Controls(r.Context(), filter)
	if err != nil {
		serverErr(w, err)
		return
	}
	if list == nil {
		list = []postgres.ControlRow{}
	}
	ok(w, list)
}

func (h *LogHandler) Stats(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	start, end = defaultTimeRange(start, end)

	list, err := h.repo.Stats(r.Context(), deviceID, start, end)
	if err != nil {
		serverErr(w, err)
		return
	}
	if list == nil {
		list = []postgres.LogStat{}
	}
	ok(w, list)
}

// ExportTelemetry 导出遥测日志为CSV
func (h *LogHandler) ExportTelemetry(w http.ResponseWriter, r *http.Request) {
	r2 := r.Clone(r.Context())
	q := r2.URL.Query()
	q.Set("export", "csv")
	r2.URL.RawQuery = q.Encode()
	h.Telemetry(w, r2)
}

// ExportControls 导出控制日志为CSV
func (h *LogHandler) ExportControls(w http.ResponseWriter, r *http.Request) {
	r2 := r.Clone(r.Context())
	q := r2.URL.Query()
	q.Set("export", "csv")
	r2.URL.RawQuery = q.Encode()
	h.Controls(w, r2)
}