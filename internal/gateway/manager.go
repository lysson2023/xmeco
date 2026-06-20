package gateway

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"xmeco/internal/gateway/transport"
)

type PollerFn func(ctx context.Context, gw *Gateway, dev DeviceRef) error

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
	NodeAddr   uint16
}

type Manager struct {
	gateways       map[string]*Gateway
	mu             sync.RWMutex
	customListener net.Listener
	dtuListener    net.Listener
	poller         PollerFn
}

func NewManager(poller PollerFn) *Manager {
	return &Manager{poller: poller, gateways: make(map[string]*Gateway)}
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
	m.pollLoop(ctx, gw)
}

func (m *Manager) handleDTUConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil || n < 1 { slog.Warn("DTU registration read failed", "remote", conn.RemoteAddr()); return }
	head := strings.TrimSpace(string(buf[:n]))
	id := "dtu-" + conn.RemoteAddr().String()
	if strings.HasPrefix(head, "GWID:") { id = strings.TrimPrefix(head, "GWID:") }
	slog.Info("DTU gateway registered", "id", id)

	t := transport.NewTransparentTransport(conn, id)
	gw := &Gateway{ID: id, Transport: t}
	m.mu.Lock()
	if old, ok := m.gateways[id]; ok { old.Transport.Close() }
	m.gateways[id] = gw
	m.mu.Unlock()
	m.pollLoop(ctx, gw)
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
			for _, dev := range gw.Devices {
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
