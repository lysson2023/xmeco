package transport

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// Custom protocol frame constants
const (
	customFrameStart = 0x68 // Frame start marker
	customFrameEnd   = 0x16 // Frame end marker
	customCmdHi      = 0xE4 // Data command byte high
	customCmdLo      = 0xA1 // Data command byte low
)

// CustomTransport handles the 0x68/0x16 proprietary gateway protocol
type CustomTransport struct {
	conn     net.Conn
	mac      []byte
	macStr   string
	timeout  time.Duration
	mu       sync.Mutex
	closed   bool
}

func NewCustomTransport(conn net.Conn, mac []byte) *CustomTransport {
	return &CustomTransport{
		conn:    conn,
		mac:     mac,
		macStr:  hex.EncodeToString(mac),
		timeout: 10 * time.Second,
	}
}

func (t *CustomTransport) Type() GatewayType { return TypeCustom }
func (t *CustomTransport) GatewayID() string { return t.macStr }

// IsConnected checks connection health by performing a non-blocking read probe.
func (t *CustomTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn == nil || t.closed {
		return false
	}
	// Read 1 byte with 1ms deadline to detect broken pipe without consuming
	// meaningful data (the byte is discarded).
	t.conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	buf := make([]byte, 1)
	_, err := t.conn.Read(buf)
	if err != nil {
		// Timeout means connection is alive; any other error means dead.
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true
		}
		slog.Warn("custom transport connection lost", "gw", t.macStr, "err", err)
		return false
	}
	return true
}

func (t *CustomTransport) SendAndReceive(data []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.conn == nil {
		return nil, fmt.Errorf("custom: connection closed")
	}
	// Wrap: 0x68 + MAC(6) + 0xE4 0xA1 + len(2) + data + checksum(2) + 0x16
	frame := t.wrap(data)
	t.conn.SetWriteDeadline(time.Now().Add(t.timeout))
	if _, err := t.conn.Write(frame); err != nil {
		t.closed = true
		return nil, fmt.Errorf("custom send: %w", err)
	}

	// Read response
	buf := make([]byte, 1024)
	t.conn.SetReadDeadline(time.Now().Add(t.timeout))
	n, err := t.conn.Read(buf)
	if err != nil {
		t.closed = true
		return nil, fmt.Errorf("custom recv: %w", err)
	}

	// Unwrap: verify 0x68/0x16, checksum, extract modbus payload
	return t.unwrap(buf[:n])
}

func (t *CustomTransport) wrap(data []byte) []byte {
	// Total: 1(0x68) + 6(MAC) + 2(cmd) + 2(len) + data + 2(chk) + 1(0x16)
	n := len(data)
	frame := make([]byte, 14+n)
	frame[0] = 0x68
	copy(frame[1:7], t.mac)
	frame[7] = customCmdHi
	frame[8] = customCmdLo
	frame[9] = byte(n >> 8)
	frame[10] = byte(n)
	copy(frame[11:11+n], data)
	sum := uint16(0)
	for i := 0; i < 11+n; i++ {
		sum += uint16(frame[i])
	}
	frame[11+n] = byte(sum >> 8)
	frame[12+n] = byte(sum)
	frame[13+n] = 0x16
	return frame
}

func (t *CustomTransport) unwrap(raw []byte) ([]byte, error) {
	n := len(raw)
	if n < 14 { return nil, fmt.Errorf("frame too short: %d", n) }
	if raw[0] != 0x68 || raw[n-1] != 0x16 {
		return nil, fmt.Errorf("invalid frame markers")
	}
	if raw[7] != customCmdHi || raw[8] != customCmdLo {
		return nil, fmt.Errorf("invalid command code")
	}
	// Verify checksum
	dataLen := int(raw[9])<<8 | int(raw[10])
	if 11+dataLen+3 > n { return nil, fmt.Errorf("length mismatch") }
	sum := uint16(0)
	for i := 0; i < 11+dataLen; i++ {
		sum += uint16(raw[i])
	}
	expected := uint16(raw[11+dataLen])<<8 | uint16(raw[12+dataLen])
	if sum != expected {
		return nil, fmt.Errorf("checksum mismatch")
	}
	return raw[11 : 11+dataLen], nil
}

func (t *CustomTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}
