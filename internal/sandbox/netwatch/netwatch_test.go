//go:build linux && (amd64 || arm64)

package netwatch

import (
	"encoding/binary"
	"testing"

	"golang.org/x/sys/unix"
)

func TestFilterProgramIsWellFormed(t *testing.T) {
	p := program()
	last := len(p) - 1
	for i, in := range p {
		if in.Code&0x07 == unix.BPF_JMP { // every jump lands inside the program
			for _, off := range []int{int(in.Jt), int(in.Jf)} {
				if i+1+off > last {
					t.Errorf("instruction %d jumps past the end", i)
				}
			}
		}
	}
	rets := map[uint32]bool{}
	for _, in := range p[last-2:] {
		if in.Code != unix.BPF_RET|unix.BPF_K {
			t.Fatalf("the last three instructions must be returns: %+v", in)
		}
		rets[in.K] = true
	}
	if !rets[retAllow] || !rets[retUserNotif] || !rets[retErrno|uint32(unix.ENOSYS)] {
		t.Errorf("missing a return: %v", rets)
	}
	// Every watched and denied call must reach the right return.
	target := func(i int, jt bool) uint32 {
		in := p[i]
		off := int(in.Jf)
		if jt {
			off = int(in.Jt)
		}
		return p[i+1+off].K
	}
	for i := 4; i < 4+len(watched); i++ {
		if target(i, true) != retUserNotif {
			t.Errorf("watched call at %d does not go to the supervisor", i)
		}
	}
	for i := 4 + len(watched); i < 4+len(watched)+len(denied); i++ {
		if target(i, true) != retErrno|uint32(unix.ENOSYS) {
			t.Errorf("denied call at %d is not refused", i)
		}
	}
}

func mem(m map[uint64][]byte) func(uint64, int) []byte {
	return func(addr uint64, n int) []byte {
		if n <= 0 { // the real reader refuses these too
			return nil
		}
		b := m[addr]
		if n < len(b) {
			b = b[:n]
		}
		return b
	}
}

func inet(port uint16, ip [4]byte) []byte {
	return append([]byte{2, 0, byte(port >> 8), byte(port)}, append(ip[:], make([]byte, 8)...)...)
}

func msghdr(name uint64, nameLen uint32) []byte {
	b := make([]byte, 64)
	binary.LittleEndian.PutUint64(b, name)
	binary.LittleEndian.PutUint32(b[8:], nameLen)
	return b
}

func TestDestinationRefusesAndRecords(t *testing.T) {
	m := mem(map[uint64][]byte{100: inet(443, [4]byte{1, 2, 3, 4}), 200: append([]byte{1, 0}, []byte("/var/run/docker.sock")...)})
	if a, e := destination(m, "connect", 100, 16, nil); e != unix.ENETUNREACH || a.Addr != "1.2.3.4" {
		t.Errorf("%+v %v", a, e)
	}
	if a, e := destination(m, "connect", 200, 40, nil); e != unix.EACCES || a.Family != "unix" {
		t.Errorf("%+v %v", a, e)
	}
	if a, e := destination(m, "sendto", 0, 0, nil); a != nil || e != 0 {
		t.Error("no destination goes on unchanged")
	}
	if _, e := destination(m, "connect", 999, 16, nil); e == 0 {
		t.Error("an unreadable address must not be allowed")
	}
}

func TestSendmmsgChecksEveryMessageNotJustTheFirst(t *testing.T) {
	var unixAddr = append([]byte{1, 0}, []byte("/run/host.sock")...)
	array := append(msghdr(0, 0), msghdr(300, uint32(len(unixAddr)))...) // message 0 has no name; message 1 names a host socket
	m := mem(map[uint64][]byte{1000: array, 300: unixAddr})
	// Reading message i at a[1]+64*i through the fake memory:
	read := func(addr uint64, n int) []byte {
		if addr >= 1000 && addr < 1000+uint64(len(array)) {
			b := array[addr-1000:]
			if n < len(b) {
				b = b[:n]
			}
			return b
		}
		return m(addr, n)
	}
	var refused bool
	for i := 0; i < 2; i++ {
		if _, e := fromMsghdr(read, "sendmmsg", 1000+uint64(i)*64); e != 0 {
			refused = true
		}
	}
	if !refused {
		t.Error("the second message names a host socket and must be refused")
	}
}

func TestHugeAddressLengthDoesNotPanic(t *testing.T) {
	m := mem(map[uint64][]byte{100: inet(80, [4]byte{9, 9, 9, 9})})
	huge := uint64(0xFFFFFFFFFFFFFFFF)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	// What judge does with a raw register value: the kernel's addrlen is 32 bits.
	destination(m, "connect", 100, int(uint32(huge)), nil)
	destination(m, "connect", 100, -1, nil)
}
