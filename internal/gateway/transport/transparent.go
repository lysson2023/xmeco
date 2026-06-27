package transport

import (
	"fmt"
	"net"
	"sync"
	"time"

	"xmeco/internal/gateway/modbus"
)

// TransparentTransport handles raw Modbus RTU over TCP (e.g., G770 DTU)
type TransparentTransport struct {
	conn         net.Conn
	id           string // IP:Port or registered ID
	timeout      time.Duration
	mu           sync.Mutex
	closed       bool
	lastActivity time.Time // tracks last successful I/O for connection health
}

func NewTransparentTransport(conn net.Conn, id string) *TransparentTransport {
	return &TransparentTransport{
		conn:         conn,
		id:           id,
		timeout:      10 * time.Second,
		lastActivity: time.Now(),
	}
}

func (t *TransparentTransport) Type() GatewayType { return TypeTransparent }
func (t *TransparentTransport) GatewayID() string { return t.id }

// IsConnected checks connection health using last-activity timestamp.
// This avoids consuming real data bytes from the stream (which the old 1-byte
// read probe did, causing protocol-level corruption on unsolicited data).
func (t *TransparentTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn == nil || t.closed {
		return false
	}
	return time.Since(t.lastActivity) < t.timeout*2
}

func (t *TransparentTransport) SendAndReceive(data []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.conn == nil {
		return nil, fmt.Errorf("dtu: connection closed")
	}

	// Drain any stale bytes BEFORE sending (previous failed read leftovers).
	// Use a single deadline to bound total drain time instead of resetting per
	// iteration (avoids unbounded drain on a trickle-feed connection).
	t.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	junk := make([]byte, 1024)
	for {
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
		n, err := t.conn.Read(buf[total:])
		if n > 0 { total += n }
		if err != nil {
			if ne, ok := err.(net.Error); !ok || !ne.Timeout() { break }
		}
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
		n, err := t.conn.Read(buf[total:])
		if n > 0 { total += n }
		if err != nil {
			if ne, ok := err.(net.Error); !ok || !ne.Timeout() { break }
		}
	}

	t.lastActivity = time.Now()

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
