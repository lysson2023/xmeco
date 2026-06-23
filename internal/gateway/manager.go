package gateway

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"xmeco/internal/gateway/transport"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PollerFn func(ctx context.Context, gw *Gateway, dev DeviceRef) error

// DeviceLoaderFn loads devices belonging to a gateway from the database.
type DeviceLoaderFn func(ctx context.Context, gatewayID string) ([]DeviceRef, error)

type Gateway struct {
	ID        string
	Transport transport.Transport
	Devices   []DeviceRef
	mu        sync.RWMutex
}

type DeviceRef struct {
	DeviceID   int
	DeviceNo   byte
	DeviceType string
	DeviceName string
	NodeAddr   uint16
}

type Manager struct {
	gateways       map[string]*Gateway
	mu             sync.RWMutex
	customListener net.Listener
	dtuListener    net.Listener
	poller         PollerFn
	deviceLoader   DeviceLoaderFn
	pool           *pgxpool.Pool
}

func NewManager(poller PollerFn, loader DeviceLoaderFn, pool *pgxpool.Pool) *Manager {
	return &Manager{poller: poller, deviceLoader: loader, pool: pool, gateways: make(map[string]*Gateway)}
}

func (m *Manager) StartCustomListener(ctx context.Context, addr string) error {
	var err error
	m.customListener, err = net.Listen("tcp", addr)
	if err != nil { return fmt.Errorf("custom listener on %s: %w", addr, err) }
	slog.Info("custom gateway listener started", "addr", addr)
	go func() {
		for {
			conn, err := m.customListener.Accept()
			if err != nil {
				select { case <-ctx.Done(): return; default: slog.Error("custom accept error", "error", err); continue }
			}
			go m.handleCustomConn(ctx, conn)
		}
	}()
	return nil
}

func (m *Manager) StartDTUListener(ctx context.Context, addr string) error {
	var err error
	m.dtuListener, err = net.Listen("tcp", addr)
	if err != nil { return fmt.Errorf("DTU listener on %s: %w", addr, err) }
	slog.Info("DTU gateway listener started", "addr", addr)
	go func() {
		for {
			conn, err := m.dtuListener.Accept()
			if err != nil {
				select { case <-ctx.Done(): return; default: slog.Error("DTU accept error", "error", err); continue }
			}
			go m.handleDTUConn(ctx, conn)
		}
	}()
	return nil
}

func (m *Manager) handleCustomConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil || n < 12 { slog.Warn("custom registration read failed", "remote", conn.RemoteAddr()); return }
	if buf[0] != 0x68 || buf[n-1] != 0x16 || buf[7] != 0xE5 || buf[8] != 0xFF {
		slog.Warn("custom invalid registration", "remote", conn.RemoteAddr()); return
	}
	mac := make([]byte, 6)
	copy(mac, buf[1:7])
	macStr := hex.EncodeToString(mac)
	slog.Info("custom gateway registered", "mac", macStr)

	t := transport.NewCustomTransport(conn, mac)
	gw := &Gateway{ID: macStr, Transport: t}
	m.mu.Lock()
	if old, ok := m.gateways[macStr]; ok { old.Transport.Close() }
	m.gateways[macStr] = gw
	m.mu.Unlock()

	// Load devices from DB
	if m.deviceLoader != nil {
		devs, err := m.deviceLoader(ctx, macStr)
		if err != nil {
			slog.Warn("load devices failed", "gw", macStr, "err", err)
		} else {
			gw.mu.Lock()
			gw.Devices = devs
			gw.mu.Unlock()
			slog.Info("devices loaded", "gw", macStr, "count", len(devs))
			// Mark all devices online on gateway connect
			m.markOnline(ctx, devs)
		}
	}

	m.pollLoop(ctx, gw)
}

func (m *Manager) handleDTUConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil || n < 1 { slog.Warn("DTU registration read failed", "remote", conn.RemoteAddr()); return }
	head := strings.TrimSpace(string(buf[:n]))
	// Resolve DTU identity: prefer GWID prefix, then look up by remote IP in gateway_config
	id := ""
	if s, ok := strings.CutPrefix(head, "GWID:"); ok {
		id = s
	} else {
		// Extract IP without port from remote address
		remoteAddr := conn.RemoteAddr().String()
		remoteIP := remoteAddr
		if host, _, err2 := net.SplitHostPort(remoteAddr); err2 == nil {
			remoteIP = host
		}
		// Look up gateway_config for DTU entries matching this IP.
		// Use exact match on dtu_ip_expected to avoid substring false positives
		// (e.g. 192.168.1.1 matching 192.168.1.10).
		if m.pool != nil {
			var gwImei string
			if err2 := m.pool.QueryRow(ctx,
				`SELECT gateway_imei FROM gateway_config
				 WHERE gateway_type='dtu'
				   AND (dtu_ip_expected = $1
				      OR dtu_ip_expected LIKE $2
				      OR dtu_ip_expected LIKE $3)
				 LIMIT 1`,
				remoteIP, remoteIP+",%", "%,"+remoteIP+",%").Scan(&gwImei); err2 == nil {
				id = gwImei
				slog.Info("DTU resolved by IP", "remote", remoteIP, "imei", id)
			}
		}
		if id == "" {
			id = "dtu-" + remoteAddr
		}
	}
	slog.Info("DTU gateway registered", "id", id)

	t := transport.NewTransparentTransport(conn, id)
	gw := &Gateway{ID: id, Transport: t}
	m.mu.Lock()
	if old, ok := m.gateways[id]; ok { old.Transport.Close() }
	m.gateways[id] = gw
	m.mu.Unlock()

	// Load devices from DB
	if m.deviceLoader != nil {
		devs, err := m.deviceLoader(ctx, id)
		if err != nil {
			slog.Warn("load devices failed", "gw", id, "err", err)
		} else {
			gw.mu.Lock()
			gw.Devices = devs
			gw.mu.Unlock()
			slog.Info("devices loaded", "gw", id, "count", len(devs))
			// Mark all devices online on DTU connect
			m.markOnline(ctx, devs)
		}
	}

	m.pollLoop(ctx, gw)
}

// markOnline updates online_status for all devices of a gateway to '在线'.
func (m *Manager) markOnline(ctx context.Context, devs []DeviceRef) {
	if m.pool == nil {
		return
	}
	for _, d := range devs {
		if _, err := m.pool.Exec(ctx, `UPDATE device SET online_status='在线', last_online_at=NOW() WHERE id=$1`, d.DeviceID); err != nil {
			slog.Warn("mark online failed", "dev", d.DeviceID, "err", err)
		}
	}
}

func (m *Manager) pollLoop(ctx context.Context, gw *Gateway) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	defer func() {
		m.mu.Lock(); delete(m.gateways, gw.ID); m.mu.Unlock()
		slog.Info("gateway disconnected", "id", gw.ID)
	}()
	for {
		select {
		case <-ctx.Done(): return
		case <-ticker.C:
			if !gw.Transport.IsConnected() { return }
			gw.mu.RLock()
			devs := gw.Devices
			gw.mu.RUnlock()
			for _, dev := range devs {
				if err := m.poller(ctx, gw, dev); err != nil {
					slog.Debug("poll error", "gw", gw.ID, "dev", dev.DeviceID, "err", err)
				}
			}
		}
	}
}

func (m *Manager) Shutdown() {
	if m.customListener != nil { m.customListener.Close() }
	if m.dtuListener != nil { m.dtuListener.Close() }
	m.mu.Lock(); defer m.mu.Unlock()
	for _, gw := range m.gateways { gw.Transport.Close() }
}

func (m *Manager) GetGateway(id string) *Gateway { m.mu.RLock(); defer m.mu.RUnlock(); return m.gateways[id] }

func (m *Manager) ListGateways() []string {
	m.mu.RLock(); defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.gateways))
	for id := range m.gateways { ids = append(ids, id) }
	return ids
}

// HandleListGateways is an HTTP handler that returns connected gateway IDs as JSON.
func (m *Manager) HandleListGateways(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(m.ListGateways())
	fmt.Fprintf(w, `{"gateways":%s}`, b)
}
