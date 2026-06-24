package intelligence

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"
)

// ---- Power Quality DTOs ----

// PowerQualityResult holds the full power quality analysis for one device.
type PowerQualityResult struct {
	DeviceID     int               `json:"device_id"`
	DeviceName   string            `json:"device_name"`
	DeviceType   string            `json:"device_type"`
	StartTime    string            `json:"start_time"`
	EndTime      string            `json:"end_time"`
	RecordCount  int               `json:"record_count"`
	Voltage      *VoltageQuality   `json:"voltage"`
	Current      *CurrentQuality   `json:"current"`
	Power        *PowerStats       `json:"power"`
	Frequency    *FrequencyQuality `json:"frequency"`
	PowerFactor  *PFStats          `json:"power_factor"`
	Harmonics    *HarmonicQuality  `json:"harmonics,omitempty"`
	Summary      string            `json:"summary"`
	OverallGrade string            `json:"overall_grade"`
}

// VoltageQuality covers voltage deviation and balance.
type VoltageQuality struct {
	AvgV           float64 `json:"avg_v"`
	MinV           float64 `json:"min_v"`
	MaxV           float64 `json:"max_v"`
	NominalV       float64 `json:"nominal_v"`
	DeviationPct   float64 `json:"deviation_pct"`
	QualifiedRate  float64 `json:"qualified_rate"`
	PhaseImbalance float64 `json:"phase_imbalance,omitempty"`
	Samples        int     `json:"samples"`
	Grade          string  `json:"grade"`
}

// CurrentQuality covers per-phase current and balance.
type CurrentQuality struct {
	AvgA            float64 `json:"avg_a"`
	AvgB            float64 `json:"avg_b"`
	AvgC            float64 `json:"avg_c"`
	MaxPhaseCurrent float64 `json:"max_phase_current"`
	ImbalancePct    float64 `json:"imbalance_pct"`
	Samples         int     `json:"samples"`
	Grade           string  `json:"grade"`
}

// PowerStats covers active/reactive/apparent power.
type PowerStats struct {
	AvgActive   float64 `json:"avg_active_kw"`
	MaxActive   float64 `json:"max_active_kw"`
	MinActive   float64 `json:"min_active_kw"`
	AvgReactive float64 `json:"avg_reactive_kvar,omitempty"`
	AvgApparent float64 `json:"avg_apparent_kva,omitempty"`
	Samples     int     `json:"samples"`
}

// PFStats covers power factor.
type PFStats struct {
	AvgPF         float64 `json:"avg_pf"`
	MinPF         float64 `json:"min_pf"`
	MaxPF         float64 `json:"max_pf"`
	QualifiedRate float64 `json:"qualified_rate"`
	Samples       int     `json:"samples"`
	Grade         string  `json:"grade"`
}

// FrequencyQuality covers grid frequency.
type FrequencyQuality struct {
	AvgHz         float64 `json:"avg_hz"`
	MinHz         float64 `json:"min_hz"`
	MaxHz         float64 `json:"max_hz"`
	NominalHz     float64 `json:"nominal_hz"`
	DeviationPct  float64 `json:"deviation_pct"`
	QualifiedRate float64 `json:"qualified_rate"`
	Samples       int     `json:"samples"`
	Grade         string  `json:"grade"`
}

// HarmonicQuality covers THD.
type HarmonicQuality struct {
	AvgTHDV float64 `json:"avg_thd_v"`
	MaxTHDV float64 `json:"max_thd_v"`
	AvgTHDI float64 `json:"avg_thd_i"`
	MaxTHDI float64 `json:"max_thd_i"`
	Samples int     `json:"samples"`
	Grade   string  `json:"grade"`
}

// ---- Grade helpers ----

func voltageGrade(deviation, qualifiedRate float64) string {
	if deviation <= 5 && qualifiedRate >= 95 {
		return "优"
	}
	if deviation <= 7 && qualifiedRate >= 85 {
		return "良"
	}
	return "差"
}

func currentGrade(imbalance float64) string {
	if imbalance <= 10 {
		return "优"
	}
	if imbalance <= 20 {
		return "良"
	}
	return "差"
}

func pfGrade(avgPF, qualifiedRate float64) string {
	if avgPF >= 0.95 && qualifiedRate >= 90 {
		return "优"
	}
	if avgPF >= 0.85 {
		return "良"
	}
	return "差"
}

func freqGrade(deviation float64) string {
	if deviation <= 0.5 {
		return "优"
	}
	if deviation <= 1.0 {
		return "良"
	}
	return "差"
}

func harmonicGrade(thdV, thdI float64) string {
	worst := math.Max(thdV, thdI)
	if worst <= 3 {
		return "优"
	}
	if worst <= 5 {
		return "良"
	}
	return "差"
}

func overallGrade(grades ...string) string {
	if len(grades) == 0 {
		return ""
	}
	diff := 0
	good := 0
	for _, g := range grades {
		switch g {
		case "差":
			diff++
		case "优":
			good++
		}
	}
	if diff > 0 {
		return "差"
	}
	if good >= len(grades)-1 {
		return "优"
	}
	return "良"
}

// ---- Analysis ----

// AnalyzePowerQuality runs power quality analysis for a device over a time range.
func (s *Service) AnalyzePowerQuality(ctx context.Context, deviceID int, startTime, endTime time.Time) (*PowerQualityResult, error) {
	var deviceName, deviceType string
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(name,''), COALESCE(device_type,'') FROM device WHERE id=$1`, deviceID,
	).Scan(&deviceName, &deviceType)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	result := &PowerQualityResult{
		DeviceID:   deviceID,
		DeviceName: deviceName,
		DeviceType: deviceType,
		StartTime:  startTime.Format(time.RFC3339),
		EndTime:    endTime.Format(time.RFC3339),
	}

	rows, err := s.pool.Query(ctx,
		`SELECT metric, value FROM device_telemetry
		 WHERE device_id=$1 AND ts >= $2 AND ts <= $3
		 ORDER BY ts`, deviceID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("telemetry query failed: %w", err)
	}
	defer rows.Close()

	var voltagesA, voltagesB, voltagesC []float64
	var currentsA, currentsB, currentsC []float64
	var activePowers, reactivePowers, apparentPowers []float64
	var powerFactors []float64
	var frequencies []float64
	var thdV, thdI []float64

	for rows.Next() {
		var metric string
		var value float64
		if err := rows.Scan(&metric, &value); err != nil {
			slog.Warn("AnalyzePowerQuality scan failed", "err", err)
			continue
		}
		switch metric {
		case "电压A", "voltage_a", "VA":
			voltagesA = append(voltagesA, value)
		case "电压B", "voltage_b", "VB":
			voltagesB = append(voltagesB, value)
		case "电压C", "voltage_c", "VC":
			voltagesC = append(voltagesC, value)
		case "电流A", "current_a", "IA":
			currentsA = append(currentsA, value)
		case "电流B", "current_b", "IB":
			currentsB = append(currentsB, value)
		case "电流C", "current_c", "IC":
			currentsC = append(currentsC, value)
		case "有功功率", "active_power", "P":
			activePowers = append(activePowers, value)
		case "无功功率", "reactive_power", "Q":
			reactivePowers = append(reactivePowers, value)
		case "视在功率", "apparent_power", "S":
			apparentPowers = append(apparentPowers, value)
		case "功率因数", "power_factor", "PF":
			powerFactors = append(powerFactors, value)
		case "频率", "frequency", "F":
			frequencies = append(frequencies, value)
		case "谐波THDV", "thd_v":
			thdV = append(thdV, value)
		case "谐波THDI", "thd_i":
			thdI = append(thdI, value)
		}
	}

	result.RecordCount = len(voltagesA) + len(currentsA) + len(activePowers) +
		len(powerFactors) + len(frequencies) + len(thdV) + len(thdI)

	var grades []string

	// --- Voltage Analysis ---
	allV := append(append(voltagesA, voltagesB...), voltagesC...)
	if len(allV) > 0 {
		vq := analyzeVoltage(allV, 220.0)
		if len(voltagesA) > 0 && len(voltagesB) > 0 && len(voltagesC) > 0 {
			vq.PhaseImbalance = calcPhaseImbalance(voltagesA, voltagesB, voltagesC)
		}
		vq.Grade = voltageGrade(vq.DeviationPct, vq.QualifiedRate)
		result.Voltage = vq
		grades = append(grades, vq.Grade)
	}

	// --- Current Analysis ---
	if len(currentsA) > 0 || len(currentsB) > 0 || len(currentsC) > 0 {
		cq := &CurrentQuality{
			AvgA: round2(avg(currentsA)),
			AvgB: round2(avg(currentsB)),
			AvgC: round2(avg(currentsC)),
		}
		cq.ImbalancePct = calcPhaseImbalance(currentsA, currentsB, currentsC)
		cq.MaxPhaseCurrent = round2(math.Max(math.Max(cq.AvgA, cq.AvgB), cq.AvgC))
		cq.Samples = len(currentsA) + len(currentsB) + len(currentsC)
		cq.Grade = currentGrade(cq.ImbalancePct)
		result.Current = cq
		grades = append(grades, cq.Grade)
	}

	// --- Power Analysis ---
	if len(activePowers) > 0 {
		result.Power = analyzePower(activePowers, reactivePowers, apparentPowers)
	}

	// --- Power Factor ---
	if len(powerFactors) > 0 {
		pf := analyzePF(powerFactors)
		pf.Grade = pfGrade(pf.AvgPF, pf.QualifiedRate)
		result.PowerFactor = pf
		grades = append(grades, pf.Grade)
	}

	// --- Frequency ---
	if len(frequencies) > 0 {
		fq := analyzeFrequency(frequencies, 50.0)
		fq.Grade = freqGrade(fq.DeviationPct)
		result.Frequency = fq
		grades = append(grades, fq.Grade)
	}

	// --- Harmonics ---
	if len(thdV) > 0 || len(thdI) > 0 {
		hq := analyzeHarmonics(thdV, thdI)
		hq.Grade = harmonicGrade(hq.AvgTHDV, hq.AvgTHDI)
		result.Harmonics = hq
		grades = append(grades, hq.Grade)
	}

	result.OverallGrade = overallGrade(grades...)
	result.Summary = pqSummary(result)

	return result, nil
}

// ---- Analysis helpers ----

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func minMax(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	mn, mx := vals[0], vals[0]
	for _, v := range vals[1:] {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	return mn, mx
}

func analyzeVoltage(vals []float64, nominal float64) *VoltageQuality {
	mn, mx := minMax(vals)
	avgV := avg(vals)
	deviation := math.Abs(avgV-nominal) / nominal * 100
	qualified := 0
	for _, v := range vals {
		if math.Abs(v-nominal)/nominal <= 0.07 {
			qualified++
		}
	}
	return &VoltageQuality{
		AvgV:          round2(avgV),
		MinV:          round2(mn),
		MaxV:          round2(mx),
		NominalV:      nominal,
		DeviationPct:  round2(deviation),
		QualifiedRate: round2(float64(qualified) / float64(len(vals)) * 100),
		Samples:       len(vals),
	}
}

func calcPhaseImbalance(a, b, c []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(c) == 0 {
		return 0
	}
	avgA, avgB, avgC := avg(a), avg(b), avg(c)
	allAvg := (avgA + avgB + avgC) / 3
	if allAvg == 0 {
		return 0
	}
	maxDev := math.Max(math.Abs(avgA-allAvg), math.Max(math.Abs(avgB-allAvg), math.Abs(avgC-allAvg)))
	return round2(maxDev / allAvg * 100)
}

func analyzePower(active, reactive, apparent []float64) *PowerStats {
	mn, mx := minMax(active)
	return &PowerStats{
		AvgActive:   round2(avg(active)),
		MaxActive:   round2(mx),
		MinActive:   round2(mn),
		AvgReactive: round2(avg(reactive)),
		AvgApparent: round2(avg(apparent)),
		Samples:     len(active),
	}
}

func analyzePF(vals []float64) *PFStats {
	mn, mx := minMax(vals)
	qualified := 0
	for _, v := range vals {
		if v >= 0.9 {
			qualified++
		}
	}
	return &PFStats{
		AvgPF:         round2(avg(vals)),
		MinPF:         round2(mn),
		MaxPF:         round2(mx),
		QualifiedRate: round2(float64(qualified) / float64(len(vals)) * 100),
		Samples:       len(vals),
	}
}

func analyzeFrequency(vals []float64, nominal float64) *FrequencyQuality {
	mn, mx := minMax(vals)
	avgF := avg(vals)
	deviation := math.Abs(avgF-nominal) / nominal * 100
	qualified := 0
	for _, v := range vals {
		if math.Abs(v-nominal) <= 1.0 {
			qualified++
		}
	}
	return &FrequencyQuality{
		AvgHz:         round2(avgF),
		MinHz:         round2(mn),
		MaxHz:         round2(mx),
		NominalHz:     nominal,
		DeviationPct:  round2(deviation),
		QualifiedRate: round2(float64(qualified) / float64(len(vals)) * 100),
		Samples:       len(vals),
	}
}

func analyzeHarmonics(thdV, thdI []float64) *HarmonicQuality {
	h := &HarmonicQuality{Samples: len(thdV) + len(thdI)}
	if len(thdV) > 0 {
		_, mxV := minMax(thdV)
		h.AvgTHDV = round2(avg(thdV))
		h.MaxTHDV = round2(mxV)
	}
	if len(thdI) > 0 {
		_, mxI := minMax(thdI)
		h.AvgTHDI = round2(avg(thdI))
		h.MaxTHDI = round2(mxI)
	}
	return h
}

func pqSummary(r *PowerQualityResult) string {
	parts := make([]string, 0, 3)

	if r.Voltage != nil {
		parts = append(parts, fmt.Sprintf("电压质量: %s (偏差%.1f%%, 合格率%.0f%%)",
			r.Voltage.Grade, r.Voltage.DeviationPct, r.Voltage.QualifiedRate))
	}

	if r.PowerFactor != nil {
		parts = append(parts, fmt.Sprintf("功率因数: %s (平均%.2f)", r.PowerFactor.Grade, r.PowerFactor.AvgPF))
	}

	if r.Frequency != nil {
		parts = append(parts, fmt.Sprintf("频率偏差: %.2f%%", r.Frequency.DeviationPct))
	}

	if r.Current != nil && r.Current.ImbalancePct > 0 {
		parts = append(parts, fmt.Sprintf("三相不平衡度: %.1f%%", r.Current.ImbalancePct))
	}

	if r.Harmonics != nil {
		parts = append(parts, fmt.Sprintf("THD-V: %.1f%%, THD-I: %.1f%%", r.Harmonics.AvgTHDV, r.Harmonics.AvgTHDI))
	}

	if len(parts) == 0 {
		return "暂无可分析的电能质量数据。请确认电表已配置遥测点位（电压、电流、功率、功率因数等）。"
	}
	return "分析结论: " + joinParts(parts)
}

func joinParts(parts []string) string {
	var b strings.Builder
	b.WriteString(parts[0])
	for i := 1; i < len(parts); i++ {
		b.WriteString("; ")
		b.WriteString(parts[i])
	}
	return b.String()
}

// ListMeters returns electric meter devices, optionally filtered by building.
func (s *Service) ListMeters(ctx context.Context, buildingID int) ([]MeterInfo, error) {
	q := `SELECT d.id, COALESCE(d.name,''), COALESCE(b.name,''), COALESCE(p.name,'')
		 FROM device d
		 LEFT JOIN building b ON b.id=d.building_id
		 LEFT JOIN project p ON p.id=b.project_id
		 WHERE d.device_type='电表'`
	var args []any
	if buildingID > 0 {
		q += ` AND d.building_id=$1`
		args = append(args, buildingID)
	}
	q += ` ORDER BY d.id`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var meters []MeterInfo
	for rows.Next() {
		var m MeterInfo
		if err := rows.Scan(&m.ID, &m.Name, &m.Building, &m.Project); err != nil {
			slog.Warn("ListMeters scan failed", "err", err)
			continue
		}
		meters = append(meters, m)
	}
	if meters == nil {
		meters = []MeterInfo{}
	}
	return meters, rows.Err()
}

// MeterInfo is a lightweight device reference for the meter selector.
type MeterInfo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Building string `json:"building"`
	Project  string `json:"project"`
}
