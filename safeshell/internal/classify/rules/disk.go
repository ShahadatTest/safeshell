package rules

import (
	"strings"

	"safeshell/internal/shellparse"
)

func ruleDd(c shellparse.Command, f flags) []Finding {
	for _, a := range f.all {
		if v, ok := strings.CutPrefix(a, "of="); ok {
			if IsBlockDevice(v) {
				return one(fnd(R4, c, "Writes raw data straight onto a disk. Everything on "+v+" will be destroyed.").
					WithAlt("lsblk", "check which disk is which before writing anything"))
			}
			return one(fnd(R3, c, "Overwrites "+v+" with raw data."))
		}
	}
	return one(fnd(R0, c, "Reads data and writes it to the screen."))
}

func ruleWipefs(c shellparse.Command, f flags) []Finding {
	if f.has('a', "all") || f.has('f', "force") {
		return one(fnd(R4, c, "Erases filesystem signatures from a disk, making its data unreadable."))
	}
	return one(fnd(R0, c, "Only lists filesystem signatures."))
}

func ruleFdisk(c shellparse.Command, f flags) []Finding {
	if f.has('l', "list") {
		return one(fnd(R0, c, "Only lists partitions."))
	}
	return one(fnd(R4, c, "Edits the partition table. A mistake can make a whole disk unreadable."))
}

func ruleParted(c shellparse.Command, f flags) []Finding {
	if f.has('l', "list") || f.hasAny("print") {
		return one(fnd(R0, c, "Only lists partitions."))
	}
	return one(fnd(R4, c, "Edits the partition table. A mistake can make a whole disk unreadable."))
}

func rulePerm(c shellparse.Command, f flags) []Finding {
	recursive := f.has('R', "recursive")
	var out []Finding
	for _, p := range f.pos {
		if ok, what := Critical(p); ok && recursive {
			out = append(out, fnd(R4, c, "Changes permissions or owners across "+what+". This can break logins, sudo and the whole system.").
				WithAlt("chmod -R u+rwX,go-w "+Shq(p), "adds only what you need, and does not open files to everyone"))
		}
	}
	if len(out) > 0 {
		return out
	}
	insecure := c.Name == "chmod" && f.hasAny("777", "666", "a+rwx", "ugo+rwx")
	switch {
	case recursive && insecure:
		return one(fnd(R2, c, "Gives everyone full access to every file in the folder. This is a security hole.").
			WithAlt("chmod -R u+rwX,go-w "+targetsAfterMode(f), "grants access to you only"))
	case recursive:
		return one(fnd(R2, c, "Changes permissions or owners of many files at once. The old values are not saved."))
	case insecure:
		return one(fnd(R2, c, "Gives everyone full access to this file. This is a security risk."))
	}
	return one(fnd(R1, c, "Changes the permissions or owner of a file."))
}

func targetsAfterMode(f flags) string {
	if len(f.pos) > 1 {
		return strings.Join(f.pos[1:], " ")
	}
	return "."
}
