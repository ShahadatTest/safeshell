package rules

import (
	"strings"

	"safeshell/internal/shellparse"
)

func ruleKill(c shellparse.Command, f flags) []Finding {
	for _, p := range f.pos {
		if p == "-1" || p == "1" || p == "0" {
			return one(fnd(R4, c, "Signals every process you own (or the init process), which can log you out or crash the machine."))
		}
	}
	// "-1" is parsed as an option, so look at the raw words too.
	if f.hasAny("-1") {
		return one(fnd(R4, c, "Signals every process you own, which can log you out or crash the machine."))
	}
	if f.hasAny("-9", "-KILL", "-SIGKILL", "-s9") || f.long["signal"] == "9" {
		return one(fnd(R2, c, "Force-kills a process. It gets no chance to save work or clean up.").
			WithAlt("kill "+strings.Join(f.pos, " "), "asks the program to quit politely first"))
	}
	return one(fnd(R1, c, "Asks a process to stop."))
}

func rulePkill(c shellparse.Command, f flags) []Finding {
	alt := "pgrep -a " + strings.Join(f.pos, " ")
	return one(fnd(R2, c, "Stops every process that matches a name or pattern, which may include more than you expect.").
		WithAlt(alt, "lists what would match, without stopping anything"))
}

func rulePower(c shellparse.Command, f flags) []Finding {
	if f.hasAny("-c") {
		return one(fnd(R1, c, "Cancels a scheduled shutdown."))
	}
	return one(fnd(R3, c, "Shuts down or restarts the machine. Running work and remote sessions end."))
}

func ruleSystemctl(c shellparse.Command, f flags) []Finding {
	switch f.sub() {
	case "poweroff", "reboot", "halt", "kexec", "suspend", "hibernate":
		return one(fnd(R3, c, "Shuts down or restarts the machine. Running work and remote sessions end."))
	case "stop", "disable", "mask", "kill", "restart", "isolate", "daemon-reexec":
		return one(fnd(R2, c, "Stops or disables a service. Anything that depends on it stops working."))
	case "status", "show", "list-units", "list-unit-files", "is-active", "is-enabled", "cat", "list-dependencies", "list-timers":
		return one(fnd(R0, c, "Only reads service status."))
	}
	return one(fnd(R1, c, "Changes a service."))
}

func ruleService(c shellparse.Command, f flags) []Finding {
	if len(f.pos) > 1 && (f.pos[1] == "stop" || f.pos[1] == "restart") {
		return one(fnd(R2, c, "Stops or restarts a service. Anything that depends on it is affected."))
	}
	return one(fnd(R1, c, "Changes a service."))
}

func ruleFirewall(c shellparse.Command, f flags) []Finding {
	if f.hasAny("-F", "--flush", "-X", "disable", "reset", "flush") {
		return one(fnd(R3, c, "Clears or disables the firewall. On a remote server this can lock you out or expose it."))
	}
	return one(fnd(R2, c, "Changes the firewall rules."))
}

func ruleCrontab(c shellparse.Command, f flags) []Finding {
	switch {
	case f.has('r', ""):
		return one(fnd(R3, c, "Deletes ALL scheduled jobs for the user, with no confirmation.").WithAlt("crontab -l > ~/crontab.bak", "saves a copy first"))
	case f.has('l', ""):
		return one(fnd(R0, c, "Only lists scheduled jobs."))
	}
	return one(fnd(R1, c, "Edits scheduled jobs."))
}

func ruleSysctl(c shellparse.Command, f flags) []Finding {
	if f.has('w', "write") || strings.Contains(f.joined(), "=") {
		return one(fnd(R2, c, "Changes a live kernel setting."))
	}
	return one(fnd(R0, c, "Only reads kernel settings."))
}

func ruleShell(c shellparse.Command, f flags) []Finding {
	return one(fnd(R1, c, "Runs another shell or script. Commands inside it are checked separately where visible."))
}
