package report

import (
	"testing"

	"safeshell/internal/domain"
	"safeshell/internal/sandbox"
)

func TestNetEffect(t *testing.T) {
	if _, ok := NetEffect(nil); ok {
		t.Error("no attempts, no effect")
	}
	if _, ok := NetEffect([]sandbox.NetAttempt{{Family: "unix", Addr: "/var/run/nscd/socket", Count: 4}}); ok {
		t.Error("libc's nscd probe is noise")
	}
	e, ok := NetEffect([]sandbox.NetAttempt{
		{Syscall: "connect", Family: "inet", Addr: "1.2.3.4", Port: 443, Count: 1},
		{Syscall: "connect", Family: "unix", Addr: "/var/run/docker.sock", Count: 1},
	})
	if !ok || e.Kind != domain.EffectNetwork || e.Evidence != domain.Observed || e.MinLevel != domain.R2 {
		t.Fatalf("%+v", e)
	}
	if len(e.Details) != 2 || e.Details[0] != "connect to 1.2.3.4:443" {
		t.Errorf("details: %v", e.Details)
	}
}
