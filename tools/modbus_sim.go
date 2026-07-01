//go:build ignore
// +build ignore

// modbus_sim simulates a 4G DTU with 6 Modbus slave devices (2 cooling towers + 4 valves).
// Usage: go run tools/modbus_sim.go [host:port] [gwid]
//   Default: go run tools/modbus_sim.go 127.0.0.1:502 dtu-gw-001

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
	gwID := "dtu-gw-001"
	if len(os.Args) > 2 {
		gwID = os.Args[2]
	}

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Printf("ERROR: cannot connect to %s: %v\n", addr, err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "GWID:%s\n", gwID)
	fmt.Printf("Connected to %s, registered as %s\n", addr, gwID)

	// Register data per device address: devAddr → { regAddr → [hi, lo] }
	// All devices use read_code=03 (Holding Register), 1 register per property.
	type devRegs map[uint16][]byte
	regs := map[byte]devRegs{
		// ---- 冷却塔1 (Modbus addr=1) ----
		0x01: {
			0x0000: {0x00, 0x01}, // 运行状态 = 01 (运行)
			0x0001: {0x01, 0xF4}, // 风机频率 = 500 → 50.0Hz
			0x0002: {0x00, 0x2D}, // 进出水温差 = 45 → 4.5°C
		},
		// ---- 冷却塔2 (Modbus addr=2) ----
		0x02: {
			0x0000: {0x00, 0x01}, // 运行状态 = 01 (运行)
			0x0001: {0x01, 0x90}, // 风机频率 = 400 → 40.0Hz
			0x0002: {0x00, 0x23}, // 进出水温差 = 35 → 3.5°C
		},
		// ---- 阀门1 (Modbus addr=3) ----
		0x03: {
			0x0000: {0x00, 0x50}, // 开度反馈 = 80 → 80%
			0x0001: {0x00, 0x01}, // 开关状态 = 01 (开)
		},
		// ---- 阀门2 (Modbus addr=4) ----
		0x04: {
			0x0000: {0x00, 0x3C}, // 开度反馈 = 60 → 60%
			0x0001: {0x00, 0x01}, // 开关状态 = 01 (开)
		},
		// ---- 阀门3 (Modbus addr=5) ----
		0x05: {
			0x0000: {0x00, 0x00}, // 开度反馈 = 0 → 0%
			0x0001: {0x00, 0x02}, // 开关状态 = 02 (关)
		},
		// ---- 阀门4 (Modbus addr=6) ----
		0x06: {
			0x0000: {0x00, 0x64}, // 开度反馈 = 100 → 100%
			0x0001: {0x00, 0x01}, // 开关状态 = 01 (开)
		},
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

		devAddr := buf[0]
		funcCode := buf[1]
		startAddr := uint16(buf[2])<<8 | uint16(buf[3])
		count := uint16(buf[4])<<8 | uint16(buf[5])

		// Look up registers for this device address
		devReg, ok := regs[devAddr]
		if !ok {
			fmt.Printf("Unknown device addr=0x%02X, skipping\n", devAddr)
			continue
		}

		// Build response: echo slave addr + func code
		// Handle error response for unsupported function codes
		if funcCode != 0x03 && funcCode != 0x04 {
			// Return exception: illegal function
			resp := []byte{devAddr, funcCode | 0x80, 0x01}
			crc := modbus.CRC16(resp)
			resp = append(resp, byte(crc), byte(crc>>8))
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			conn.Write(resp)
			fmt.Printf("Poll: addr=%d func=%02X → exception(illegal function)\n", devAddr, funcCode)
			continue
		}

		byteCount := byte(count * 2)
		resp := []byte{devAddr, funcCode, byteCount}

		for i := uint16(0); i < count; i++ {
			if b, ok := devReg[startAddr+i]; ok {
				resp = append(resp, b...)
			} else {
				resp = append(resp, 0x00, 0x00)
			}
		}

		crc := modbus.CRC16(resp)
		resp = append(resp, byte(crc), byte(crc>>8))

		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		conn.Write(resp)
		fmt.Printf("Poll: addr=%d func=%02X start=%d count=%d → %d bytes CRC=%04X\n",
			devAddr, funcCode, startAddr, count, len(resp), crc)
	}
}
