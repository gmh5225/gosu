package ipc

import (
	"context"
	"net"
)

func Dial(address string) (conn net.Conn, err error) {
	return DialContext(context.Background(), address)
}
