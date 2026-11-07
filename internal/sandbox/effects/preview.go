package effects

import (
	"fmt"
	"strings"

	"github.com/samarthsinh2660/safeshell/internal/domain"
	"github.com/samarthsinh2660/safeshell/internal/shellparse"
)

// Runner runs a read-only program and returns its output. Production code runs
// real programs with a short timeout; tests supply canned output.
type Runner func(name string, args ...string) (string, error)

// essentialPackages are ones whose removal breaks the machine or its login.
var essentialPackages = map[string]bool{
	"systemd": true, "libc6": true, "bash": true, "sudo": true, "apt": true, "dpkg": true, "coreutils": true,
	"ubuntu-desktop": true, "ubuntu-minimal": true, "ubuntu-standard": true, "gnome-shell": true, "xorg": true,
	"network-manager": true, "netplan.io": true, "grub-pc": true, "grub-efi-amd64": true, "login": true,
	"linux-image-generic": true, "openssh-server": true, "init": true, "udev": true, "dbus": true,
}

// PreviewPackages asks apt itself what an install, removal or purge would do,
// using its built-in simulation (-s), which changes nothing and needs no root.
func PreviewPackages(run Runner, cmds []shellparse.Command) []domain.Effect {
	var out []domain.Effect
	for _, c := range cmds {
		if c.Name != "apt" && c.Name != "apt-get" {
			continue
		}
		sub := firstPositional(c.Args)
		if sub != "remove" && sub != "purge" && sub != "autoremove" && sub != "autopurge" {
			continue
		}
		args := append([]string{"-s"}, withoutAssumeYes(c.Args)...)
		text, err := run("apt-get", args...)
		if err != nil && text == "" {
			continue // apt is not here or refused the arguments; the rules still apply
		}
		removed := parseRemoved(text)
		e := domain.Effect{Kind: domain.EffectPackages, Evidence: domain.Previewed, MinLevel: domain.R2}
		var essential []string
		for _, p := range removed {
			if essentialPackages[p] {
				essential = append(essential, p)
			}
		}
		switch {
		case len(essential) > 0:
			e.MinLevel = domain.R4
			e.Summary = fmt.Sprintf("apt would remove %d packages, including essential ones: %s", len(removed), strings.Join(essential, ", "))
		case len(removed) >= 25:
			e.MinLevel = domain.R3
			e.Summary = fmt.Sprintf("apt would remove %d packages, far more than the one you named", len(removed))
		case len(removed) == 0:
			e.Summary = "apt would remove nothing"
		default:
			e.Summary = fmt.Sprintf("apt would remove %d package(s)", len(removed))
		}
		for i, p := range removed {
			if i == 12 {
				e.Details = append(e.Details, fmt.Sprintf("... and %d more", len(removed)-12))
				break
			}
			e.Details = append(e.Details, "remove "+p)
		}
		out = append(out, e)
	}
	return out
}

func firstPositional(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

func withoutAssumeYes(args []string) []string {
	var out []string
	for _, a := range args {
		if a != "-y" && a != "--yes" && a != "--assume-yes" && a != "-s" && a != "--simulate" {
			out = append(out, a)
		}
	}
	return out
}

// parseRemoved reads apt's "Remv name [version]" lines.
func parseRemoved(text string) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if f := strings.Fields(l); len(f) >= 2 && f[0] == "Remv" {
			out = append(out, f[1])
		}
	}
	return out
}

// criticalUnits are services whose loss locks you out or takes the desktop down.
var criticalUnits = map[string]string{
	"dbus.service": "the message bus almost everything depends on", "systemd-logind.service": "logins and sessions",
	"display-manager.service": "the graphical login and your desktop session", "gdm.service": "the graphical login and your desktop session",
	"NetworkManager.service": "the network connection", "sshd.service": "remote logins over SSH", "ssh.service": "remote logins over SSH",
	"systemd-journald.service": "system logging", "systemd-udevd.service": "device detection",
}

// InspectServices asks systemd which running services depend on the unit being
// stopped, disabled or masked. It only reads.
func InspectServices(run Runner, cmds []shellparse.Command) []domain.Effect {
	var out []domain.Effect
	for _, c := range cmds {
		if c.Name != "systemctl" {
			continue
		}
		sub := firstPositional(c.Args)
		if sub != "stop" && sub != "disable" && sub != "mask" && sub != "kill" && sub != "restart" {
			continue
		}
		for _, unit := range unitsAfter(c.Args, sub) {
			if !strings.Contains(unit, ".") {
				unit += ".service"
			}
			e := domain.Effect{Kind: domain.EffectServices, Evidence: domain.Inspected, MinLevel: domain.R2}
			if why, ok := criticalUnits[unit]; ok && sub != "restart" {
				e.MinLevel = domain.R3
				e.Details = append(e.Details, unit+" provides "+why)
			}
			deps := runningDependents(run, unit)
			for i, d := range deps {
				if i == 10 {
					e.Details = append(e.Details, fmt.Sprintf("... and %d more", len(deps)-10))
					break
				}
				e.Details = append(e.Details, d+" is running and depends on it")
			}
			if len(deps) >= 10 {
				e.MinLevel = max(e.MinLevel, domain.R3)
			}
			e.Summary = fmt.Sprintf("%s %s: %d running service(s) depend on it", sub, unit, len(deps))
			out = append(out, e)
		}
	}
	return out
}

func unitsAfter(args []string, sub string) []string {
	var out []string
	seen := false
	for _, a := range args {
		switch {
		case !seen && a == sub:
			seen = true
		case seen && !strings.HasPrefix(a, "-"):
			out = append(out, a)
		}
	}
	return out
}

// runningDependents lists the active units that need unit, per systemd.
func runningDependents(run Runner, unit string) []string {
	text, err := run("systemctl", "list-dependencies", "--reverse", "--plain", "--no-pager", unit)
	if err != nil && text == "" {
		return nil
	}
	var cands []string
	for i, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if i == 0 || l == "" || !strings.HasSuffix(l, ".service") || l == unit {
			continue
		}
		cands = append(cands, l)
	}
	if len(cands) == 0 {
		return nil
	}
	states, _ := run("systemctl", append([]string{"is-active"}, cands...)...)
	var active []string
	for i, s := range strings.Split(strings.TrimSpace(states), "\n") {
		if i < len(cands) && strings.TrimSpace(s) == "active" {
			active = append(active, cands[i])
		}
	}
	return active
}
