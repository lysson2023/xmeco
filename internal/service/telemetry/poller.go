package telemetry

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"xmeco/internal/domain"
	"xmeco/internal/gateway"
	"xmeco/internal/gateway/modbus"
	"xmeco/internal/gateway/transport"
	"xmeco/internal/repository/postgres"
	"xmeco/internal/service/alarm"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Poller struct {
	pool     *pgxpool.Pool
	alarm    *alarm.Engine
	devRepo  *postgres.DeviceRepo
	regRepo  *postgres.RegisterRepo
	propRepo *postgres.PropertyRepo
}

func NewPoller(pool *pgxpool.Pool) *Poller {
	return &Poller{
		pool:     pool,
		alarm:    alarm.New(pool),
		devRepo:  postgres.NewDeviceRepo(pool),
		regRepo:  postgres.NewRegisterRepo(pool),
		propRepo: postgres.NewPropertyRepo(pool),
	}
}

func (p *Poller) PollDevice(ctx context.Context, gw *gateway.Gateway, dev gateway.DeviceRef) error {
	registers, err := p.regRepo.ListByDeviceID(ctx, dev.DeviceID)
	if err != nil { return fmt.Errorf("registers: %w", err) }
	if len(registers) == 0 { return nil }

	props, err := p.propRepo.List(ctx, dev.DeviceID)
	if err != nil { return fmt.Errorf("properties: %w", err) }
	propMap := make(map[int]*domain.DeviceProperty)
	for i := range props { propMap[props[i].ID] = &props[i] }

	tt := gw.Transport
	now := time.Now()

	type pt struct{ DeviceID int; Metric string; Value float64; Unit string }
	var points []pt

	for _, reg := range registers {
		var raw []byte
		mb := modbus.BuildReadCommand(dev.DeviceNo, codeFromStr(reg.ReadCode), uint16(reg.ReadAddr), uint16(reg.DataLength))
		if tt.Type() == transport.TypeCustom {
			lora := []byte{byte(dev.NodeAddr >> 8), byte(dev.NodeAddr)}
			raw, err = tt.SendAndReceive(append(lora, mb...))
		} else { raw, err = tt.SendAndReceive(mb) }
		if err != nil { slog.Debug("poll", "dev", dev.DeviceID, "err", err); continue }
		data, ok := modbus.ParseResponse(raw)
		if !ok { continue }
		val := decodeVal(data, reg)

		// Update device status from status_code mapping (e.g. "01=运行,02=停机")
		if reg.StatusCode != nil && *reg.StatusCode != "" {
			if statusLabel, ok := parseStatusMapping(*reg.StatusCode, val); ok {
				if _, err := p.pool.Exec(ctx, `UPDATE device SET device_status=$1 WHERE id=$2`, statusLabel, dev.DeviceID); err != nil {
					slog.Warn("update device_status failed", "dev", dev.DeviceID, "err", err)
				}
			}
		}

		prop, ok := propMap[reg.PropertyID]
		if !ok { continue }
		metric := prop.PropType
		if metric == "" { metric = prop.PropName }
		points = append(points, pt{dev.DeviceID, metric, val, prop.Unit})
	}

	if len(points) > 0 {
		batch := &pgx.Batch{}
		for _, pt := range points {
			batch.Queue(`INSERT INTO device_telemetry (ts,device_id,metric,value,unit) VALUES($1,$2,$3,$4,$5)`,
				now, pt.DeviceID, pt.Metric, pt.Value, pt.Unit)
		}
		br := p.pool.SendBatch(ctx, batch)
		defer br.Close()
		for range points {
			if _, err := br.Exec(); err != nil {
				slog.Warn("telemetry batch insert failed", "dev", dev.DeviceID, "err", err)
			}
		}

		// Mark device online — any successful read confirms connectivity
		if _, err := p.pool.Exec(ctx, `UPDATE device SET online_status='在线', last_online_at=NOW() WHERE id=$1`, dev.DeviceID); err != nil {
			slog.Warn("update online_status failed", "dev", dev.DeviceID, "err", err)
		}

		// Evaluate alarm rules for each metric
		for _, pt := range points {
			p.alarm.Evaluate(ctx, pt.DeviceID, dev.DeviceName, dev.DeviceType, pt.Metric, pt.Value)
		}
	}
	return nil
}

// codeFromStr maps a Modbus function code hex string to byte.
// Supported read codes: 01 (coils), 02 (discrete), 03 (holding, default), 04 (input).
// Custom codes fall back to 03 (Read Holding Registers) to avoid write on read.
func codeFromStr(s string) byte {
	switch s {
	case "01": return 0x01
	case "02": return 0x02
	case "04": return 0x04
	case "03": return 0x03
	default:
		// attempt hex parse for custom codes like "0F", "17"
		if v, err := hexToByte(s); err == nil {
			return v
		}
		return 0x03 // safe default: Read Holding Registers
	}
}

func hexToByte(s string) (byte, error) {
	var b byte
	for _, c := range s {
		b <<= 4
		switch {
		case c >= '0' && c <= '9': b |= byte(c - '0')
		case c >= 'a' && c <= 'f': b |= byte(c - 'a' + 10)
		case c >= 'A' && c <= 'F': b |= byte(c - 'A' + 10)
		default: return 0, fmt.Errorf("invalid hex char: %c", c)
		}
	}
	return b, nil
}

func decodeVal(data []byte, reg domain.Register) float64 {
	if len(data) == 0 { return 0 }
	dt := reg.DataType

	// Reorder bytes based on data_order before decoding
	data = reorderForOrder(data, reg.DataOrder)

	// Single-precision float (IEEE 754, 4 bytes)
	if isFloat(dt) && len(data) >= 4 {
		bits := binary.BigEndian.Uint32(data[:4])
		return float64(math.Float32frombits(bits))
	}

	// Integer decode
	var raw uint64
	switch len(data) {
	case 1: raw = uint64(data[0])
	case 2: raw = uint64(binary.BigEndian.Uint16(data))
	case 4: raw = uint64(binary.BigEndian.Uint32(data))
	default: raw = uint64(binary.BigEndian.Uint16(data[:2]))
	}
	// Apply bit mask (e.g. "0001" extracts LSB, "00FF" extracts low byte)
	raw = applyMask(raw, reg.DataMask)
	val := float64(raw) / reg.Magnification
	if isSigned(dt) {
		switch len(data) {
		case 2: val = float64(int16(raw)) / reg.Magnification
		case 4: val = float64(int32(raw)) / reg.Magnification
		}
	}
	return math.Round(val*1000) / 1000
}

// reorderForOrder reorders data bytes based on the configured byte/word order.
//
// "高位在前" (default): Big Endian   — [0x12,0x34] → 0x1234  (keep as-is)
// "低位在前": Little Endian           — [0x12,0x34] → [0x34,0x12] → 0x3412
// "低字在前": Low word first (32bit) — [Hi,Hi,Lo,Lo] → [Lo,Lo,Hi,Hi]
func reorderForOrder(data []byte, order string) []byte {
	switch order {
	case "低位在前":
		// Reverse bytes within each 16-bit register
		for i := 0; i+1 < len(data); i += 2 {
			data[i], data[i+1] = data[i+1], data[i]
		}
	case "低字在前":
		// Reverse 16-bit words (for 32-bit/4-byte values)
		if len(data) >= 4 {
			data[0], data[2] = data[2], data[0]
			data[1], data[3] = data[3], data[1]
		}
	}
	return data
}

// applyMask applies a hex bitmask to the raw value.
// Example masks: "0001" (extract LSB), "00FF" (extract low byte), "" (no-op).
func applyMask(raw uint64, mask *string) uint64 {
	if mask == nil || *mask == "" {
		return raw
	}
	m, err := parseHex(*mask)
	if err != nil {
		return raw
	}
	return raw & m
}

func parseHex(s string) (uint64, error) {
	var v uint64
	for _, c := range s {
		v <<= 4
		switch {
		case c >= '0' && c <= '9': v |= uint64(c - '0')
		case c >= 'a' && c <= 'f': v |= uint64(c - 'a' + 10)
		case c >= 'A' && c <= 'F': v |= uint64(c - 'A' + 10)
		default: return 0, fmt.Errorf("invalid hex: %c", c)
		}
	}
	return v, nil
}

func isFloat(dt string) bool {
	return strings.Contains(dt, "float") || strings.Contains(dt, "浮点") || strings.Contains(dt, "Float")
}

func isSigned(dt string) bool {
	return strings.Contains(dt, "signed") || strings.Contains(dt, "s16") || strings.Contains(dt, "s32") ||
		strings.Contains(dt, "int16") || strings.Contains(dt, "int32") ||
		strings.Contains(dt, "有符号")
}

// parseStatusMapping parses a status_code string like "01=运行,02=停机,03=故障"
// and returns the label matching the raw register value (converted to hex).
func parseStatusMapping(statusCode string, rawVal float64) (string, bool) {
	hexKey := fmt.Sprintf("%02X", int(rawVal))
	for _, pair := range strings.Split(statusCode, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 && strings.EqualFold(kv[0], hexKey) {
			return kv[1], true
		}
	}
	return "", false
}
