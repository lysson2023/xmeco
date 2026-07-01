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

// BuildWriteSingleCommand builds func 05/06 - write single coil/register
func BuildWriteSingleCommand(devAddr byte, funcCode byte, addr uint16, value uint16) []byte {
	buf := make([]byte, 8)
	buf[0] = devAddr
	buf[1] = funcCode
	buf[2] = byte(addr >> 8)
	buf[3] = byte(addr)
	buf[4] = byte(value >> 8)
	buf[5] = byte(value)
	crc := CRC16(buf[:6])
	buf[6] = byte(crc)
	buf[7] = byte(crc >> 8)
	return buf
}

// BuildWriteMultiCommand builds func 0x0F (write multiple coils) or 0x10 (write multiple registers).
// funcCode must be 0x0F or 0x10. For 0x0F, data is packed as bits (1 bit per coil); count is the number of coils.
// For 0x10, data is register values (2 bytes per register); count is the number of registers.
func BuildWriteMultiCommand(devAddr byte, funcCode byte, addr uint16, count uint16, data []byte) []byte {
	var dataLenBytes int
	var bufLen int
	if funcCode == 0x0F {
		// Write Multiple Coils: dataLen is in bytes (ceil(count/8))
		dataLenBytes = (int(count) + 7) / 8
		bufLen = 9 + dataLenBytes
	} else {
		// Write Multiple Registers (0x10): dataLen is count * 2
		dataLenBytes = int(count) * 2
		bufLen = 9 + dataLenBytes
	}
	buf := make([]byte, bufLen)
	buf[0] = devAddr
	buf[1] = funcCode
	buf[2] = byte(addr >> 8)
	buf[3] = byte(addr)
	buf[4] = byte(count >> 8)
	buf[5] = byte(count)
	buf[6] = byte(dataLenBytes)
	// 只拷贝 min(len(data), dataLenBytes) 防止溢出
	copyLen := len(data)
	if copyLen > dataLenBytes {
		copyLen = dataLenBytes
	}
	copy(buf[7:], data[:copyLen])
	crc := CRC16(buf[:7+dataLenBytes])
	buf[7+dataLenBytes] = byte(crc)
	buf[8+dataLenBytes] = byte(crc >> 8)
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
	case 0x05, 0x06, 0x0F:
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
	case "10":
		return 0x10
	case "0F":
		return 0x0F
	default:
		return 0x06
	}
}

// BuildWriteCommand builds a Modbus write command based on the function code.
// For 0x05/0x06 (single coil/register): value is the 16-bit value to write.
// For 0x0F/0x10 (multiple coils/registers): value is written count times;
// use BuildWriteMultiCommand directly if you need per-register data.
func BuildWriteCommand(devAddr byte, funcCode byte, addr uint16, count uint16, value uint16) []byte {
	switch funcCode {
	case 0x05, 0x06:
		return BuildWriteSingleCommand(devAddr, funcCode, addr, value)
	case 0x10, 0x0F:
		// Construct data array: repeat the 16-bit value count times
		data := make([]byte, int(count)*2)
		for i := uint16(0); i < count; i++ {
			data[i*2] = byte(value >> 8)
			data[i*2+1] = byte(value)
		}
		return BuildWriteMultiCommand(devAddr, funcCode, addr, count, data)
	default:
		// Unknown function code: fall back to write single register (0x06)
		return BuildWriteSingleCommand(devAddr, 0x06, addr, value)
	}
}
