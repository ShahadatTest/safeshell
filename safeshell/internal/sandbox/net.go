package sandbox

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

// NetAttempt is one network operation the command tried inside the sandbox.
// The sandbox has no route out, so none of them reached anything.
type NetAttempt struct {
	Syscall string `json:"syscall"` // connect, sendto, sendmsg
	Family  string `json:"family"`  // inet, inet6, unix
	Addr    string `json:"addr"`    // IP address or unix socket path
	Port    int    `json:"port,omitempty"`
	DNS     string `json:"dns,omitempty"` // the name being looked up, when the datagram is a DNS query
	Count   int    `json:"count"`
}

const (
	afUnix  = 1
	afInet  = 2
	afInet6 = 10
)

// ParseSockaddr decodes the struct sockaddr a process passed to the kernel.
func ParseSockaddr(b []byte) (NetAttempt, bool) {
	if len(b) < 2 {
		return NetAttempt{}, false
	}
	switch binary.LittleEndian.Uint16(b) {
	case afInet:
		if len(b) < 8 {
			return NetAttempt{}, false
		}
		ip := netip.AddrFrom4([4]byte(b[4:8]))
		return NetAttempt{Family: "inet", Addr: ip.String(), Port: int(binary.BigEndian.Uint16(b[2:4]))}, true
	case afInet6:
		if len(b) < 24 {
			return NetAttempt{}, false
		}
		ip := netip.AddrFrom16([16]byte(b[8:24]))
		return NetAttempt{Family: "inet6", Addr: ip.String(), Port: int(binary.BigEndian.Uint16(b[2:4]))}, true
	case afUnix:
		path := b[2:]
		if len(path) > 0 && path[0] == 0 { // abstract socket: shown with a leading @
			path = append([]byte{'@'}, path[1:]...)
		}
		if i := strings.IndexByte(string(path), 0); i >= 0 {
			path = path[:i]
		}
		return NetAttempt{Family: "unix", Addr: string(path)}, true
	}
	return NetAttempt{}, false
}

// ParseDNSName returns the name asked for in a DNS query datagram, or "".
func ParseDNSName(p []byte) string {
	if len(p) < 13 || binary.BigEndian.Uint16(p[4:6]) == 0 { // header is 12 bytes; QDCOUNT must be > 0
		return ""
	}
	var labels []string
	for i := 12; i < len(p); {
		n := int(p[i])
		if n == 0 {
			return strings.Join(labels, ".")
		}
		if n > 63 || i+1+n > len(p) { // labels are at most 63 bytes; 0xC0 pointers do not occur in a question
			return ""
		}
		labels = append(labels, string(p[i+1:i+1+n]))
		i += 1 + n
	}
	return ""
}

// Merge combines repeated identical attempts and orders the result.
func MergeAttempts(in []NetAttempt) []NetAttempt {
	type key struct {
		Syscall, Family, Addr, DNS string
		Port                       int
	}
	idx := map[key]int{}
	var out []NetAttempt
	for _, a := range in {
		k := key{a.Syscall, a.Family, a.Addr, a.DNS, a.Port}
		if i, ok := idx[k]; ok {
			out[i].Count++
			continue
		}
		if a.Count == 0 {
			a.Count = 1
		}
		idx[k] = len(out)
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

// String describes the attempt for a report.
func (a NetAttempt) String() string {
	var s string
	switch {
	case a.DNS != "":
		s = "look up " + a.DNS
	case a.Family == "unix":
		s = "connect to local socket " + a.Addr
	default:
		s = fmt.Sprintf("%s %s", map[string]string{"connect": "connect to", "sendto": "send data to", "sendmsg": "send data to"}[a.Syscall], hostPort(a))
	}
	if a.Count > 1 {
		s += fmt.Sprintf(" (%d times)", a.Count)
	}
	return s
}

func hostPort(a NetAttempt) string {
	if a.Family == "inet6" {
		return fmt.Sprintf("[%s]:%d", a.Addr, a.Port)
	}
	return fmt.Sprintf("%s:%d", a.Addr, a.Port)
}
