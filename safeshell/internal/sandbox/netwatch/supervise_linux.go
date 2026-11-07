//go:build linux && (amd64 || arm64)

package netwatch

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	"safeshell/internal/sandbox"
)

const (
	ioctlRecv    = 0xC0502100 // SECCOMP_IOCTL_NOTIF_RECV
	ioctlSend    = 0xC0182101 // SECCOMP_IOCTL_NOTIF_SEND
	ioctlIDValid = 0x40082102 // SECCOMP_IOCTL_NOTIF_ID_VALID
	flagContinue = 1          // SECCOMP_USER_NOTIF_FLAG_CONTINUE
)

type notif struct {
	ID    uint64
	PID   uint32
	Flags uint32
	NR    int32
	Arch  uint32
	IP    uint64
	Args  [6]uint64
}

type notifResp struct {
	ID    uint64
	Val   int64
	Error int32
	Flags uint32
}

func ioctl(f *os.File, req uintptr, arg unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, uintptr(arg))
	if e != 0 {
		return e
	}
	return nil
}

// Supervise answers notifications on fd until every filtered process has
// exited, and returns what they tried to reach.
func Supervise(fd *os.File) []sandbox.NetAttempt {
	defer fd.Close()
	var seen []sandbox.NetAttempt
	for {
		var n notif
		if err := ioctl(fd, ioctlRecv, unsafe.Pointer(&n)); err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return sandbox.MergeAttempts(seen) // ENOENT: no filtered process is left
		}
		att, errno := judge(n)
		if att != nil {
			seen = append(seen, *att)
		}
		resp := notifResp{ID: n.ID}
		if errno == 0 {
			resp.Flags = flagContinue
		} else {
			resp.Error = -int32(errno)
		}
		_ = ioctl(fd, ioctlSend, unsafe.Pointer(&resp)) // fails only if the caller died meanwhile
	}
}

// judge reads the destination out of the caller's memory. A call that names
// no destination goes on unchanged; anything else is refused.
func judge(n notif) (*sandbox.NetAttempt, syscall.Errno) {
	mem, err := os.Open(fmt.Sprintf("/proc/%d/mem", n.PID))
	if err != nil {
		return nil, unix.EACCES // cannot see where it is going, so do not let it go
	}
	defer mem.Close()
	read := func(addr uint64, size int) []byte {
		b := make([]byte, size)
		got, _ := mem.ReadAt(b, int64(addr))
		return b[:got]
	}
	a := n.Args
	var name uint64
	var nameLen int
	var data []byte
	call := map[int32]string{unix.SYS_CONNECT: "connect", unix.SYS_SENDTO: "sendto", unix.SYS_SENDMSG: "sendmsg", unix.SYS_SENDMMSG: "sendmmsg"}[n.NR]
	switch n.NR {
	case unix.SYS_CONNECT:
		name, nameLen = a[1], int(a[2])
	case unix.SYS_SENDTO:
		name, nameLen = a[4], int(a[5])
		if name != 0 && a[2] > 0 {
			data = read(a[1], int(min(a[2], 512)))
		}
	default: // sendmsg, sendmmsg: msghdr (or the first mmsghdr) holds the name pointer and length
		if h := read(a[1], 16); len(h) == 16 {
			name, nameLen = binary.LittleEndian.Uint64(h), int(binary.LittleEndian.Uint32(h[8:]))
		}
	}
	if name == 0 {
		return nil, 0
	}
	att, ok := sandbox.ParseSockaddr(read(name, min(nameLen, 128)))
	if !ok {
		return &sandbox.NetAttempt{Syscall: call, Family: "other"}, unix.EAFNOSUPPORT
	}
	att.Syscall = call
	if att.Port == 53 {
		att.DNS = sandbox.ParseDNSName(data)
	}
	if att.Family == "unix" {
		return &att, unix.EACCES // a host socket (docker, ssh-agent, dbus) is a way out of the sandbox
	}
	return &att, unix.ENETUNREACH
}
