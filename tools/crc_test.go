//go:build ignore
// +build ignore

package main

import (
	"fmt"

	"xmeco/internal/gateway/modbus"
)

func main() {
	// Test vector: 01 03 02 00 DC → should produce correct CRC
	data := []byte{0x01, 0x03, 0x02, 0x00, 0xDC}
	prodCRC := modbus.CRC16(data)
	fmt.Printf("Production CRC16: %04X\n", prodCRC)

	// Simulate building a response and verifying it
	resp := []byte{0x01, 0x03, 0x02, 0x00, 0xDC}
	crc := prodCRC
	resp = append(resp, byte(crc), byte(crc>>8))
	fmt.Printf("Full response: %X\n", resp)

	if modbus.VerifyCRC(resp) {
		fmt.Println("CRC VERIFY: PASS")
	} else {
		fmt.Println("CRC VERIFY: FAIL")
	}
}
