package sandbox

import (
	"strings"
	"testing"
)

func sockaddrIn(port uint16, ip [4]byte) []byte {
	return append([]byte{2, 0, byte(port >> 8), byte(port)}, append(ip[:], make([]byte, 8)...)...)
}

func TestParseSockaddr(t *testing.T) {
	a, ok := ParseSockaddr(sockaddrIn(443, [4]byte{140, 82, 112, 3}))
	if !ok || a.Family != "inet" || a.Addr != "140.82.112.3" || a.Port != 443 {
		t.Errorf("inet: %+v %v", a, ok)
	}
	v6 := make([]byte, 28)
	v6[0], v6[2], v6[3] = 10, 0x1f, 0x90 // AF_INET6, port 8080
	v6[8+15] = 1                         // ::1
	if a, ok := ParseSockaddr(v6); !ok || a.Family != "inet6" || a.Addr != "::1" || a.Port != 8080 {
		t.Errorf("inet6: %+v %v", a, ok)
	}
	unix := append([]byte{1, 0}, []byte("/var/run/docker.sock\x00\x00\x00")...)
	if a, ok := ParseSockaddr(unix); !ok || a.Family != "unix" || a.Addr != "/var/run/docker.sock" {
		t.Errorf("unix: %+v %v", a, ok)
	}
	abstract := append([]byte{1, 0, 0}, []byte("dbus-xyz")...)
	if a, _ := ParseSockaddr(abstract); a.Addr != "@dbus-xyz" {
		t.Errorf("abstract: %+v", a)
	}
	for _, bad := range [][]byte{nil, {2}, {2, 0, 0, 80, 1, 2}, {10, 0, 0, 0}, {99, 0, 0, 0, 0, 0, 0, 0}} {
		if _, ok := ParseSockaddr(bad); ok {
			t.Errorf("%v should not parse", bad)
		}
	}
}

func dnsQuery(name string) []byte {
	b := []byte{0x12, 0x34, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	for _, l := range strings.Split(name, ".") {
		b = append(b, byte(len(l)))
		b = append(b, l...)
	}
	return append(b, 0, 0, 1, 0, 1)
}

func TestParseDNSName(t *testing.T) {
	if got := ParseDNSName(dnsQuery("api.github.com")); got != "api.github.com" {
		t.Errorf("got %q", got)
	}
	q := dnsQuery("x.io")
	q[5] = 0 // QDCOUNT 0
	for _, bad := range [][]byte{nil, make([]byte, 12), q, append(dnsQuery("a")[:12], 70), append(dnsQuery("a")[:12], 5, 'a')} {
		if got := ParseDNSName(bad); got != "" {
			t.Errorf("ParseDNSName(%v) = %q, want empty", bad, got)
		}
	}
}

func TestMergeAttemptsCountsAndOrders(t *testing.T) {
	a := NetAttempt{Syscall: "connect", Family: "inet", Addr: "1.1.1.1", Port: 443}
	b := NetAttempt{Syscall: "sendto", Family: "inet", Addr: "127.0.0.53", Port: 53, DNS: "example.com"}
	got := MergeAttempts([]NetAttempt{a, b, b, b, a})
	if len(got) != 2 || got[0].DNS != "example.com" || got[0].Count != 3 || got[1].Count != 2 {
		t.Errorf("%+v", got)
	}
	if len(MergeAttempts(nil)) != 0 {
		t.Error("nil in, nothing out")
	}
}

func TestAttemptString(t *testing.T) {
	for want, a := range map[string]NetAttempt{
		"connect to 1.2.3.4:443":                       {Syscall: "connect", Family: "inet", Addr: "1.2.3.4", Port: 443, Count: 1},
		"look up example.com (3 times)":                {Syscall: "sendto", Family: "inet", Addr: "127.0.0.53", Port: 53, DNS: "example.com", Count: 3},
		"connect to local socket /var/run/docker.sock": {Syscall: "connect", Family: "unix", Addr: "/var/run/docker.sock"},
		"send data to [::1]:9":                         {Syscall: "sendmsg", Family: "inet6", Addr: "::1", Port: 9},
	} {
		if got := a.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
