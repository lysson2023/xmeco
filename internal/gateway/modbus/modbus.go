package modbus

var crc16Table [256]uint16

func init() {
	for i := range crc16Table {
		crc := uint16(i)
		for j := 0; j < 8; j++ {
			if crc&1 == 1 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
		crc16Table[i] = crc
	}
}

// CRC16 computes Modbus CRC-16
func CRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc = (crc >> 8) ^ crc16Table[(crc^uint16(b))&0xFF]
	}
	return crc
}

// BuildReadCommand builds a Modbus read command (func 01-04)
func BuildReadCommand(devAddr byte, funcCode byte, startAddr uint16, count uint16) []byte {
	buf := make([]byte, 8)
	buf[0] = devAddr
	buf[1] = funcCode
	buf[2] = byte(startAddr >> 8)
	buf[3] = byte(startAddr)
	buf[4] = byte(count >> 8)
	buf[5] = byte(count)
	crc := CRC16(buf[:6])
	buf[6] = byte(crc)
	buf[7] = byte(crc >> 8)
	return buf
}

// BuildWriteSingleCommand builds func 06 - write single register
func BuildWriteSingleCommand(devAddr byte, addr uint16, value uint16) []byte {
	buf := make([]byte, 8)
	buf[0] = devAddr
	buf[1] = 0x06
	buf[2] = byte(addr >> 8)
	buf[3] = byte(addr)
	buf[4] = byte(value >> 8)
	buf[5] = byte(value)
	crc := CRC16(buf[:6])
	buf[6] = byte(crc)
	buf[7] = byte(crc >> 8)
	return buf
}

// BuildWriteMultiCommand builds func 10 - write multiple registers
func BuildWriteMultiCommand(devAddr byte, addr uint16, count uint16, data []byte) []byte {
	dataLen := int(count) * 2
	buf := make([]byte, 9+dataLen)
	buf[0] = devAddr
	buf[1] = 0x10
	buf[2] = byte(addr >> 8)
	buf[3] = byte(addr)
	buf[4] = byte(count >> 8)
	buf[5] = byte(count)
	buf[6] = byte(dataLen)
	copy(buf[7:], data)
	crc := CRC16(buf[:7+dataLen])
	buf[7+dataLen] = byte(crc)
	buf[8+dataLen] = byte(crc >> 8)
	return buf
}

// VerifyCRC checks Modbus RTU response CRC
func VerifyCRC(data []byte) bool {
	if len(data) < 3 { return false }
	return CRC16(data[:len(data)-2]) == (uint16(data[len(data)-1])<<8 | uint16(data[len(data)-2]))
}

// ParseResponse extracts data bytes from Modbus response
func ParseResponse(raw []byte) ([]byte, bool) {
	if len(raw) < 5 { return nil, false }
	if !VerifyCRC(raw) { return nil, false }
	funcCode := raw[1]
	switch funcCode {
	case 0x01, 0x02, 0x03, 0x04:
		byteCount := int(raw[2])
		if len(raw) < 5+byteCount { return nil, false }
		return raw[3 : 3+byteCount], true
	case 0x05, 0x06:
		return raw[2:6], true
	case 0x10:
		if len(raw) < 8 { return nil, false }
		return raw[4:6], true
	}
	return nil, false
}

// CodeFromStr converts a hex code string to a Modbus function code byte.
func CodeFromStr(s string) byte {
	switch s {
	case "01":
		return 0x01
	case "02":
		return 0x02
	case "03":
		return 0x03
	case "04":
		return 0x04
	case "05":
		return 0x05
	case "06":
		return 0x06
	case "10", "0F":
		return 0x10
	default:
		return 0x06
	}
}

// BuildWriteCommand builds a Modbus write command based on the function code.
func BuildWriteCommand(devAddr byte, funcCode byte, addr uint16, count uint16) []byte {
	switch funcCode {
	case 0x05, 0x06:
		return BuildWriteSingleCommand(devAddr, addr, 0xFF00)
	case 0x10, 0x0F:
		return BuildWriteMultiCommand(devAddr, addr, count, []byte{0xFF, 0x00})
	default:
		return BuildWriteSingleCommand(devAddr, addr, 0xFF00)
	}
}
