package handler

import (
	"encoding/json"
	"net/http"

	"xmeco/internal/api/middleware"
	"xmeco/internal/repository/postgres"
)

type AlarmHandler struct {
	ruleRepo *postgres.AlarmRuleRepo
	logRepo  *postgres.AlarmLogRepo
}

func NewAlarmHandler(ruleRepo *postgres.AlarmRuleRepo, logRepo *postgres.AlarmLogRepo) *AlarmHandler {
	return &AlarmHandler{ruleRepo: ruleRepo, logRepo: logRepo}
}

func (h *AlarmHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	deviceID := queryInt(r, "device_id")
	list, err := h.ruleRepo.List(r.Context(), deviceID)
	if err != nil {
		serverErr(w, err)
		return
	}
	if list == nil {
		list = []postgres.AlarmRule{}
	}
	ok(w, list)
}

func (h *AlarmHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	// 使用灵活的请求结构，处理enabled默认值
	var body struct {
		Name        string   `json:"name"`
		DeviceID    *int     `json:"device_id"`
		PropertyID  *int     `json:"property_id"`
		DeviceType  *string  `json:"device_type"`
		Metric      *string  `json:"metric"`
		Condition   *string  `json:"condition"`
		Threshold   *float64 `json:"threshold"`
		Level       *string  `json:"level"`
		TargetValue *string  `json:"target_value"`
		MinValue    *string  `json:"min_value"`
		MaxValue    *string  `json:"max_value"`
		NotifyUsers []int    `json:"notify_users"`
		Enabled     *bool    `json:"enabled"` // 使用指针判断是否显式设置
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, M{"error": "规则名称不能为空"})
		return
	}

	// 构造AlarmRule，处理enabled默认值
	rr := postgres.AlarmRule{
		Name:        body.Name,
		DeviceID:    body.DeviceID,
		PropertyID:  body.PropertyID,
		DeviceType:  body.DeviceType,
		Metric:      body.Metric,
		Condition:   body.Condition,
		Threshold:   body.Threshold,
		Level:       body.Level,
		TargetValue: body.TargetValue,
		MinValue:    body.MinValue,
		MaxValue:    body.MaxValue,
		NotifyUsers: body.NotifyUsers,
	}
	// 显式设置enabled：如果body中没有该字段（nil），默认为true；否则使用传入值
	if body.Enabled == nil {
		rr.Enabled = true
	} else {
		rr.Enabled = *body.Enabled
	}

	// 直接调用repo，让repo使用rr.Enabled的值
	// 但repo.Create现在总是传true，需要修改repo接受enabled参数
	// 临时方案：根据rr.Enabled构造SQL参数
	if err := h.ruleRepo.CreateWithEnabled(r.Context(), &rr, rr.Enabled); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, map[string]any{"id": rr.ID})
}

func (h *AlarmHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var rr postgres.AlarmRule
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		writeJSON(w, http.StatusBadRequest, errBadRequest)
		return
	}
	rr.ID = id
	if err := h.ruleRepo.Update(r.Context(), &rr); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status": "updated"})
}

func (h *AlarmHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if err := h.ruleRepo.Delete(r.Context(), pathID(r)); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status": "deleted"})
}

func (h *AlarmHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	filter := postgres.AlarmLogFilter{
		DeviceID:   queryInt(r, "device_id"),
		BuildingID: queryInt(r, "building_id"),
		ProjectID:  queryInt(r, "project_id"),
		DateFrom:   r.URL.Query().Get("date_from"),
		DateTo:     r.URL.Query().Get("date_to"),
		Today:      r.URL.Query().Get("today") == "1",
	}

	// 校验日期格式
	if filter.DateFrom != "" && !isValidDate(filter.DateFrom) {
		writeJSON(w, http.StatusBadRequest, M{"error": "date_from 格式无效，请使用 YYYY-MM-DD 或 ISO 8601 格式"})
		return
	}
	if filter.DateTo != "" && !isValidDate(filter.DateTo) {
		writeJSON(w, http.StatusBadRequest, M{"error": "date_to 格式无效，请使用 YYYY-MM-DD 或 ISO 8601 格式"})
		return
	}

	list, err := h.logRepo.List(r.Context(), filter)
	if err != nil {
		serverErr(w, err)
		return
	}
	if list == nil {
		list = []postgres.AlarmLog{}
	}
	ok(w, list)
}

func (h *AlarmHandler) AckLog(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	claims := middleware.GetClaims(r)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, M{"error": "未认证"})
		return
	}
	if err := h.logRepo.Ack(r.Context(), id, claims.Username); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, M{"status": "acked"})
}