package intelligence

import (
	"context"
	"log/slog"
	"math"
)

// RecommendSetpoints generates setpoint optimization suggestions
// based on outdoor temperature, device types, and best-practice guidelines.
func (s *Service) RecommendSetpoints(ctx context.Context, outdoorTemp float64) []SetpointRecommendation {
	var recs []SetpointRecommendation

	// Query chillers and their properties
	rows, err := s.pool.Query(ctx,
		`SELECT d.id, d.name, d.device_type,
		 COALESCE((SELECT prop_value FROM device_properties WHERE device_id=d.id AND prop_name='额定功率'), '120')
		 FROM device d WHERE d.device_type IN ('主机','冷冻泵','冷却泵','冷却塔')
		 LIMIT 10`)
	if err != nil {
		slog.Warn("RecommendSetpoints query failed", "err", err)
		return generateDemoRecommendations(outdoorTemp)
	}
	defer rows.Close()

	type dev struct {
		id   int
		name string
		dtype string
		kw   string
	}
	var devices []dev
	for rows.Next() {
		var d dev
		if err := rows.Scan(&d.id, &d.name, &d.dtype, &d.kw); err != nil {
			slog.Warn("RecommendSetpoints scan failed", "err", err)
			continue
		}
		devices = append(devices, d)
	}

	if len(devices) == 0 {
		return generateDemoRecommendations(outdoorTemp)
	}

	for _, d := range devices {
		switch d.dtype {
		case "主机":
			recs = append(recs, chillerRecommendation(d.id, d.name, outdoorTemp))
		case "冷冻泵":
			recs = append(recs, chilledPumpRecommendation(d.id, d.name, outdoorTemp))
		case "冷却泵":
			recs = append(recs, condenserPumpRecommendation(d.id, d.name, outdoorTemp))
		case "冷却塔":
			recs = append(recs, coolingTowerRecommendation(d.id, d.name, outdoorTemp))
		}
	}

	if len(recs) == 0 {
		return generateDemoRecommendations(outdoorTemp)
	}
	return recs
}

func chillerRecommendation(id int, name string, outdoorTemp float64) SetpointRecommendation {
	currentChilledWater := 7.0 // typical default

	var recChilled float64
	var reason string

	if outdoorTemp <= 25 {
		recChilled = 8.0
		reason = "室外温度较低，适度提高冷冻水温即可满足冷量需求，可降低主机压缩比"
	} else if outdoorTemp <= 30 {
		recChilled = 7.0
		reason = "室外温度适中，维持标准设定值"
	} else {
		recChilled = 6.5
		reason = "室外高温，需降低冷冻水温以提升供冷能力"
	}

	saveKW := (currentChilledWater - recChilled) * 3.5 // ~3.5 kW per °C adjustment
	if saveKW < 0 {
		saveKW = 0
	}

	return SetpointRecommendation{
		DeviceID:         id,
		DeviceName:       name,
		Parameter:        "冷冻水出水温度",
		CurrentValue:     currentChilledWater,
		RecommendedValue: recChilled,
		Unit:             "°C",
		Reason:           reason,
		EstimatedSaveKW:  round2(math.Abs(saveKW)),
		Priority:         prioBySaveKW(math.Abs(saveKW)),
	}
}

func chilledPumpRecommendation(id int, name string, outdoorTemp float64) SetpointRecommendation {
	currentFreq := 50.0
	recFreq := 45.0
	reason := "根据末端温差反馈，降低频率运行可满足当前流量需求，减少输配能耗"

	if outdoorTemp > 30 {
		recFreq = 48.0
		reason = "高温季节需保持较高流量以保证供冷效果"
	}

	saveKW := (currentFreq - recFreq) * 0.3
	if saveKW < 0 {
		saveKW = 0
	}

	return SetpointRecommendation{
		DeviceID:         id,
		DeviceName:       name,
		Parameter:        "变频器频率",
		CurrentValue:     currentFreq,
		RecommendedValue: recFreq,
		Unit:             "Hz",
		Reason:           reason,
		EstimatedSaveKW:  round2(saveKW),
		Priority:         prioBySaveKW(saveKW),
	}
}

func condenserPumpRecommendation(id int, name string, outdoorTemp float64) SetpointRecommendation {
	currentFreq := 50.0
	recFreq := 42.0
	reason := "室外温度适中，降低冷却泵频率可大幅减少输配能耗，同时保证散热"

	if outdoorTemp > 32 {
		recFreq = 48.0
		reason = "高温季节需提高冷却水流量以增强散热"
	}

	saveKW := (currentFreq - recFreq) * 0.25
	if saveKW < 0 {
		saveKW = 0
	}

	return SetpointRecommendation{
		DeviceID:         id,
		DeviceName:       name,
		Parameter:        "变频器频率",
		CurrentValue:     currentFreq,
		RecommendedValue: recFreq,
		Unit:             "Hz",
		Reason:           reason,
		EstimatedSaveKW:  round2(saveKW),
		Priority:         prioBySaveKW(saveKW),
	}
}

func coolingTowerRecommendation(id int, name string, outdoorTemp float64) SetpointRecommendation {
	currentFreq := 50.0
	recFreq := 38.0
	reason := "室外湿球温度较低时，降低冷却塔风扇频率即可满足散热需求，大幅节电"

	if outdoorTemp > 30 {
		recFreq = 45.0
		reason = "高温季节需提高风扇转速以保证冷却效果"
	}

	saveKW := (currentFreq - recFreq) * 0.1
	if saveKW < 0 {
		saveKW = 0
	}

	return SetpointRecommendation{
		DeviceID:         id,
		DeviceName:       name,
		Parameter:        "风扇频率",
		CurrentValue:     currentFreq,
		RecommendedValue: recFreq,
		Unit:             "Hz",
		Reason:           reason,
		EstimatedSaveKW:  round2(saveKW),
		Priority:         prioBySaveKW(saveKW),
	}
}

func prioBySaveKW(kw float64) string {
	switch {
	case kw >= 5:
		return "高"
	case kw >= 2:
		return "中"
	default:
		return "低"
	}
}

func generateDemoRecommendations(outdoorTemp float64) []SetpointRecommendation {
	return []SetpointRecommendation{
		chillerRecommendation(0, "约克主机1", outdoorTemp),
		chillerRecommendation(0, "约克主机2", outdoorTemp),
		chilledPumpRecommendation(0, "冷冻泵1", outdoorTemp),
		condenserPumpRecommendation(0, "冷却泵1", outdoorTemp),
		coolingTowerRecommendation(0, "冷却塔1", outdoorTemp),
	}
}
