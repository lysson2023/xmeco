package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"xmeco/internal/domain"
	"xmeco/internal/service/external/weather"
)

type WeatherHandler struct {
	svc *weather.Service
}

func NewWeatherHandler(svc *weather.Service) *WeatherHandler {
	return &WeatherHandler{svc: svc}
}

// ListCities 获取城市列表（支持 ?q= 搜索）
func (h *WeatherHandler) ListCities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	cities, err := h.svc.SearchCities(r.Context(), q)
	if err != nil {
		slog.Warn("weather search cities failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, M{"error": "天气服务暂不可用"})
		return
	}
	if cities == nil {
		cities = []domain.City{}
	}
	writeJSON(w, http.StatusOK, cities)
}

// ListProvinceCities 获取省→市树形列表（用于级联选择器）
func (h *WeatherHandler) ListProvinceCities(w http.ResponseWriter, r *http.Request) {
	data, err := h.svc.ListProvinceCities(r.Context())
	if err != nil {
		slog.Warn("weather list province cities failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, M{"error": "天气服务暂不可用"})
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// GetCity 获取单个城市信息
func (h *WeatherHandler) GetCity(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	city, err := h.svc.GetCity(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, M{"error": "城市不存在"})
		return
	}
	writeJSON(w, http.StatusOK, city)
}

// Now 获取实时天气 ?city_id= 或 ?city_name=
func (h *WeatherHandler) Now(w http.ResponseWriter, r *http.Request) {
	cityIDStr := r.URL.Query().Get("city_id")
	cityName := r.URL.Query().Get("city_name")

	var cityID int
	if cityIDStr != "" {
		id, err := strconv.Atoi(cityIDStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, M{"error": "city_id 格式错误"})
			return
		}
		cityID = id
	}

	if cityID == 0 && cityName == "" {
		writeJSON(w, http.StatusBadRequest, M{"error": "city_id 和 city_name 不能同时为空"})
		return
	}

	wd, err := h.svc.GetWeather(r.Context(), cityID, cityName)
	if err != nil {
		slog.Warn("weather get failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, M{"error": "天气服务暂不可用"})
		return
	}
	writeJSON(w, http.StatusOK, wd)
}

// ProjectWeather 根据项目获取天气 ?project_id=
func (h *WeatherHandler) ProjectWeather(w http.ResponseWriter, r *http.Request) {
	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		writeJSON(w, http.StatusBadRequest, M{"error": "缺少 project_id 参数"})
		return
	}

	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, M{"error": "project_id 格式错误"})
		return
	}

	wd, err := h.svc.GetProjectWeather(r.Context(), projectID)
	if err != nil {
		slog.Warn("weather project weather failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, M{"error": "天气服务暂不可用"})
		return
	}
	writeJSON(w, http.StatusOK, wd)
}


