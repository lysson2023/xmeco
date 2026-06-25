package transport

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"xmeco/internal/gateway/modbus"
)

// TransparentTransport handles raw Modbus RTU over TCP (e.g., G770 DTU)
type TransparentTransport struct {
	conn    net.Conn
	id      string // IP:Port or registered ID
	timeout time.Duration
	mu      sync.Mutex
	closed  bool
}

func NewTransparentTransport(conn net.Conn, id string) *TransparentTransport {
	return &TransparentTransport{
		conn:    conn,
		id:      id,
		timeout: 10 * time.Second,
	}
}

func (t *TransparentTransport) Type() GatewayType { return TypeTransparent }
func (t *TransparentTransport) GatewayID() string { return t.id }

// IsConnected checks connection health by performing a non-blocking read probe.
func (t *TransparentTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn == nil || t.closed {
		return false
	}
	// Read 1 byte with 1ms deadline to detect broken pipe.
	t.conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	buf := make([]byte, 1)
	_, err := t.conn.Read(buf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true
		}
		slog.Warn("dtu transport connection lost", "gw", t.id, "err", err)
		return false
	}
	return true
}

func (t *TransparentTransport) SendAndReceive(data []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.conn == nil {
		return nil, fmt.Errorf("dtu: connection closed")
	}

	// Drain any stale bytes BEFORE sending (previous failed read leftovers).
	// Limit to max 10 iterations to prevent infinite loops on noisy connections.
	t.conn.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
	junk := make([]byte, 1024)
	for i := 0; i < 10; i++ {
		_, err := t.conn.Read(junk)
		if err != nil { break }
	}

	t.conn.SetWriteDeadline(time.Now().Add(t.timeout))
	if _, err := t.conn.Write(data); err != nil {
		t.closed = true
		return nil, fmt.Errorf("dtu send: %w", err)
	}

	// Read Modbus response: addr(1) + func(1) + ...
	t.conn.SetReadDeadline(time.Now().Add(t.timeout))
	buf := make([]byte, 512)
	total := 0
	deadline := time.Now().Add(t.timeout)

	// Read at least 3 bytes (addr + func + byteCount/status)
	for total < 3 && time.Now().Before(deadline) {
		t.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _ := t.conn.Read(buf[total:])
		if n > 0 { total += n }
	}
	if total < 3 { return nil, fmt.Errorf("dtu: no response") }

	// Determine expected length
	funcCode := buf[1]
	expectedLen := 0
	switch funcCode {
	case 0x01, 0x02, 0x03, 0x04:
		if total >= 3 {
			expectedLen = 3 + int(buf[2]) + 2 // addr+func+byteCount+data+CRC
		}
	case 0x05, 0x06:
		expectedLen = 8
	case 0x10:
		expectedLen = 8
	default:
		if funcCode&0x80 != 0 { expectedLen = 5 } // error response
	}
	if expectedLen == 0 { return nil, fmt.Errorf("dtu: unknown func %02X", funcCode) }

	// Read remaining bytes
	for total < expectedLen && time.Now().Before(deadline) {
		t.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _ := t.conn.Read(buf[total:])
		if n > 0 { total += n }
	}

	result := buf[:total]
	if !modbus.VerifyCRC(result) {
		return nil, fmt.Errorf("dtu: CRC mismatch")
	}
	return result, nil
}

func (t *TransparentTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	if t.conn != nil { return t.conn.Close() }
	return nil
}
