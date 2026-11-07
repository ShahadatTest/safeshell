//go:build linux && (amd64 || arm64)

// Package netwatch records the network operations a sandboxed command attempts
// and refuses them. A seccomp filter sends connect/sendto/sendmsg/sendmmsg to a
// supervisor in the parent process, which reads the destination from the
// caller's memory and answers with an error.
package netwatch

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	retAllow     = 0x7fff0000
	retUserNotif = 0x7fc00000
	retErrno     = 0x00050000

	flagNewListener = 8
	x32Bit          = 0x40000000 // x86-64 x32 ABI syscalls carry this bit and would dodge the number checks
)

func auditArch() uint32 {
	if runtime.GOARCH == "arm64" {
		return 0xC00000B7
	}
	return 0xC000003E
}

// watched are the calls that can name a destination.
var watched = []uint32{unix.SYS_CONNECT, unix.SYS_SENDTO, unix.SYS_SENDMSG, unix.SYS_SENDMMSG}

// program builds the BPF filter: other architectures and the x32 ABI get
// ENOSYS (they could bypass the number checks), watched calls go to the
// supervisor, everything else runs normally.
func program() []unix.SockFilter {
	n := len(watched)
	notify, deny := 5+n, 6+n // indexes of the return instructions (allow is 4+n)
	jump := func(i, target int) uint8 { return uint8(target - i - 1) }
	p := []unix.SockFilter{
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 4}, // seccomp_data.arch
		{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: auditArch(), Jt: 0, Jf: jump(1, deny)},
		{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: 0}, // seccomp_data.nr
		{Code: unix.BPF_JMP | unix.BPF_JGE | unix.BPF_K, K: x32Bit, Jt: jump(3, deny), Jf: 0},
	}
	for i, nr := range watched {
		at := 4 + i
		p = append(p, unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: nr, Jt: jump(at, notify), Jf: 0})
	}
	return append(p,
		unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: retAllow},
		unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: retUserNotif},
		unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: retErrno | uint32(unix.ENOSYS)},
	)
}

// Install applies the filter to the calling thread, which stays locked, and so
// to every child started from it and returns the supervisor's notification descriptor.
// It needs no-new-privs or CAP_SYS_ADMIN.
func Install() (*os.File, error) {
	runtime.LockOSThread() // never unlocked: children are forked from this thread and inherit the filter
	prog := program()
	fprog := unix.SockFprog{Len: uint16(len(prog)), Filter: &prog[0]}
	fd, _, errno := syscall.Syscall(unix.SYS_SECCOMP, 1 /* SET_MODE_FILTER */, flagNewListener, uintptr(unsafe.Pointer(&fprog)))
	if errno != 0 {
		return nil, fmt.Errorf("seccomp filter: %w", errno)
	}
	syscall.CloseOnExec(int(fd)) // the command must not hold its own supervisor
	return os.NewFile(fd, "seccomp-notify"), nil
}
