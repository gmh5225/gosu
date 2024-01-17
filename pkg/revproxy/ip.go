package revproxy

import (
	"hash/crc32"
	"net/http"
)

var realIpHeaders = []string{
	http.CanonicalHeaderKey("CF-Connecting-IP"),
	http.CanonicalHeaderKey("X-Forwarded-For"),
	http.CanonicalHeaderKey("X-Real-Ip"),
}

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

func getClientIp(r *http.Request) string {
	for _, header := range realIpHeaders {
		if ip := r.Header.Get(header); ip != "" {
			return ip
		}
	}
	adrBytes := []byte(r.RemoteAddr)
	for i := len(adrBytes) - 1; i >= 0; i-- {
		if adrBytes[i] == ':' {
			return string(adrBytes[:i])
		}
	}
	return r.RemoteAddr
}
func fastHash(s string) int {
	chk := crc32.Checksum([]byte(s), crc32cTable)
	return int(chk)
}
