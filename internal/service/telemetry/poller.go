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
	GwMgr    *gateway.Manager
	alarm    *alarm.Engine
	devRepo  *postgres.DeviceRepo
	regRepo  *postgres.RegisterRepo
	propRepo *postgres.PropertyRepo
}

func NewPoller(pool *pgxpool.Pool, gwMgr *gateway.Manager) *Poller {
	return &Poller{
		pool:     pool,
		GwMgr:    gwMgr,
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
		for range points { br.Exec() }

		// Evaluate alarm rules for each metric
		for _, pt := range points {
			p.alarm.Evaluate(ctx, pt.DeviceID, dev.DeviceType, dev.DeviceType, pt.Metric, pt.Value)
		}
	}
	return nil
}

func codeFromStr(s string) byte {
	switch s { case "01": return 0x01; case "02": return 0x02; case "04": return 0x04; default: return 0x03 }
}

func decodeVal(data []byte, reg domain.Register) float64 {
	if len(data) == 0 { return 0 }
	var raw uint64
	switch len(data) {
	case 1: raw = uint64(data[0]); case 2: raw = uint64(binary.BigEndian.Uint16(data))
	case 4: raw = uint64(binary.BigEndian.Uint32(data)); default: raw = uint64(binary.BigEndian.Uint16(data[:2]))
	}
	val := float64(raw) / reg.Magnification
	dt := reg.DataType
	if isSigned(dt) {
		switch len(data) { case 2: val = float64(int16(raw)) / reg.Magnification; case 4: val = float64(int32(raw)) / reg.Magnification }
	}
	return math.Round(val*1000) / 1000
}

func isSigned(dt string) bool {
	return strings.Contains(dt, "signed") || strings.Contains(dt, "s16") || strings.Contains(dt, "s32") ||
		strings.Contains(dt, "int16") || strings.Contains(dt, "int32")
}
