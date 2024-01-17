//go:build windows
// +build windows

package ipc

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

func DialContext(ctx context.Context, address string) (conn net.Conn, err error) {
	return winio.DialPipeContext(ctx, address)
}
