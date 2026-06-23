package modbus

import "testing"

func TestCRC16(t *testing.T) {
	// Standard Modbus CRC vector: 01 03 00 00 00 01 -> CRC = 84 0A
	input := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01}
	crc := CRC16(input)
	if crc != 0x0A84 {
		t.Errorf("CRC16 = %04X, want 0A84", crc)
	}
}

func TestBuildReadCommand(t *testing.T) {
	cmd := BuildReadCommand(0x01, 0x03, 0x0000, 0x0001)
	if len(cmd) != 8 { t.Fatalf("len=%d want 8", len(cmd)) }
	if cmd[6] != 0x84 || cmd[7] != 0x0A {
		t.Errorf("CRC bytes = %02X %02X, want 84 0A", cmd[6], cmd[7])
	}
}

func TestVerifyCRC(t *testing.T) {
	valid := []byte{0x01, 0x03, 0x02, 0x00, 0x00, 0xB8, 0x44}
	if !VerifyCRC(valid) {
		t.Error("valid frame failed CRC check")
	}
	valid[0] = 0x02 // corrupt
	if VerifyCRC(valid) {
		t.Error("corrupt frame passed CRC check")
	}
}
