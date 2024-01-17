//go:build !windows
// +build !windows

// go:build !windows
package ipc

import (
	"context"
	"net"
)

func DialContext(ctx context.Context, address string) (conn net.Conn, err error) {
	return net.Dial("unix", address)
}
