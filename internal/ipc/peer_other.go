//go:build !linux

package ipc

import (
	"errors"
	"net"
)

func peerUID(net.Conn) (int, error) {
	return 0, errors.New("peer credentials are only available on Linux")
}
