package utils

import "encoding/binary"

// convert uint64 to bytes
func UintBytesConvert(i int64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(i))
	return b
}

//convert date to uint64
