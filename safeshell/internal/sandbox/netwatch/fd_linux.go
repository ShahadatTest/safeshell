//go:build linux && (amd64 || arm64)

package netwatch

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// Offer tells the parent which descriptor holds the listener and waits until
// the parent has taken its own copy. It must use only unfiltered calls: the
// filter is already active in this process, and sending a descriptor with
// sendmsg would wait for a supervisor that does not exist yet.
func Offer(sock, listener *os.File) error {
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(listener.Fd()))
	if _, err := sock.Write(n[:]); err != nil {
		return err
	}
	_, err := io.ReadFull(sock, make([]byte, 1))
	return err
}

// Take copies the listener out of process pid, which called Offer.
func Take(sock *os.File, pid int) (*os.File, error) {
	var n [4]byte
	if _, err := io.ReadFull(sock, n[:]); err != nil {
		return nil, err
	}
	pidfd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		return nil, fmt.Errorf("pidfd_open: %w", err)
	}
	defer unix.Close(pidfd)
	fd, err := unix.PidfdGetfd(pidfd, int(binary.LittleEndian.Uint32(n[:])), 0)
	if err != nil {
		return nil, fmt.Errorf("pidfd_getfd: %w", err)
	}
	_, _ = sock.Write([]byte{1})
	return os.NewFile(uintptr(fd), "seccomp-notify"), nil
}
