package util

import (
	"crypto/rand"
	"encoding/binary"
	mrand "math/rand"
	"time"
)

func RandomBytes(n int) []byte {
	buf := make([]byte, (n+7)&^7)
	if x, err := rand.Read(buf); err != nil || x < n {
		for i := 0; i < n; i += 8 {
			j := uint64(time.Now().UnixNano())
			j = j*6364136223846793005 + 1442695040888963407
			j ^= mrand.Uint64()
			binary.LittleEndian.PutUint64(buf[i:], j)
		}
	}
	return buf[:n]
}
