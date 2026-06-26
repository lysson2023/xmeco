//go:build ignore
// +build ignore

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"

	"xmeco/internal/gateway/modbus"
)

func main() {
	// Connect to modbus_sim
	conn, err := net.DialTimeout("tcp", "127.0.0.1:502", 3*time.Second)
	if err != nil {
		fmt.Printf("FAIL: cannot connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("OK: connected to 127.0.0.1:502")

	// Register as DTU-TEST (mimics real gateway)
	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	conn.Write([]byte("GWID:DTU-TEST\n"))

	// Wait for server to process registration
	time.Sleep(5 * time.Second)

	// Now READ from the connection — the server should send Modbus queries
	buf := make([]byte, 256)
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("FAIL: no data received: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("RECEIVED %d bytes: %X\n", n, buf[:n])

	if n < 8 {
		fmt.Println("FAIL: too short for Modbus request")
		os.Exit(1)
	}

	// Parse Modbus read request
	addr := buf[0]
	fc := buf[1]
	start := binary.BigEndian.Uint16(buf[2:4])
	count := binary.BigEndian.Uint16(buf[4:6])
	crcRx := binary.LittleEndian.Uint16(buf[6:8])

	fmt.Printf("Request: addr=%d func=%02X start=%d count=%d\n", addr, fc, start, count)

	// Verify CRC
	expectedCRC := modbus.CRC16(buf[:6])
	if crcRx != expectedCRC {
		fmt.Printf("FAIL: CRC mismatch — got %04X, want %04X\n", crcRx, expectedCRC)
		os.Exit(1)
	}
	fmt.Printf("OK: CRC matches (%04X)\n", crcRx)

	// Build response
	resp := []byte{addr, fc, byte(count * 2)}
	for i := uint16(0); i < count; i++ {
		resp = append(resp, byte((start+i)>>8), byte(start+i))
	}
	respCRC := modbus.CRC16(resp)
	resp = append(resp, byte(respCRC), byte(respCRC>>8))

	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	conn.Write(resp)
	fmt.Printf("Sent response: %d bytes, CRC=%04X\n", len(resp), respCRC)
	fmt.Println("TEST PASSED ✓")
}
