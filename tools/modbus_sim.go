//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"xmeco/internal/gateway/modbus"
)

func main() {
	addr := "127.0.0.1:502"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Printf("ERROR: cannot connect to %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "GWID:DTU-TEST\n")
	fmt.Printf("Connected to %s, registered as DTU-TEST\n", addr)

	// Register data: addr → [hi, lo]
	regs := map[uint16][]byte{
		0x0000: {0x00, 0xDC}, // 220V
		0x0002: {0x00, 0x32}, // 50A
		0x0004: {0x01, 0x00}, // 256 → 25.6°C
		0x0006: {0x01, 0x7C}, // 380V
	}

	buf := make([]byte, 256)
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("Disconnected: %v\n", err)
			return
		}
		if n < 8 {
			continue
		}

		startAddr := uint16(buf[2])<<8 | uint16(buf[3])
		count := uint16(buf[4])<<8 | uint16(buf[5])

		// Build response using PRODUCTION CRC
		var resp []byte
		resp = append(resp, 0x01)      // addr
		resp = append(resp, 0x03)      // func
		byteCount := byte(count * 2)
		resp = append(resp, byteCount) // count

		for i := uint16(0); i < count; i++ {
			if b, ok := regs[startAddr+i]; ok {
				resp = append(resp, b...)
			} else {
				resp = append(resp, 0x00, 0x00)
			}
		}

		crc := modbus.CRC16(resp)
		resp = append(resp, byte(crc), byte(crc>>8))

		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		conn.Write(resp)
		fmt.Printf("Poll: addr=%d count=%d → response %d bytes CRC=%04X\n", startAddr, count, len(resp), crc)
	}
}
