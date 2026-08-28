package check

// wire types we handle
const (
	wireVarint = 0
	wireBytes  = 2
)

func readVarint(buf []byte, i int) (uint64, int) {
	var value uint64
	var shift uint
	for i < len(buf) {
		b := buf[i]
		i++
		value |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return value, i
}

func tag(b byte) (field int, wire int) {
	return int(b >> 3), int(b & 0x07)
}
