package risk

import (
	"strings"

	"safeshell/internal/shellparse"
)

// A rule rates one command. It returns one finding per problem it sees.
type rule func(c shellparse.Command, f flags) []Finding

func fnd(l Level, c shellparse.Command, reason string) Finding {
	return Finding{Level: l, Command: c.String(), Reason: reason}
}

// alt attaches the safer command that should be suggested for this finding.
func (f Finding) alt(cmd, note string) Finding {
	f.Alt = &Alternative{Command: cmd, Note: note}
	return f
}

func one(f Finding) []Finding { return []Finding{f} }

var readOnly = strings.Fields(`ls ll la dir cat less more head tail wc grep egrep fgrep rg ag pwd cd echo printf date whoami id uname hostname
which type whereis file stat df du free uptime ps top htop printenv tree diff cmp sort uniq cut tr jq yq basename dirname realpath
readlink lsblk lscpu lsusb lspci lsof man help history true false test [ sleep clear export alias unalias set unset read
env nproc md5sum sha1sum sha256sum cksum column paste comm join fold nl rev tac seq yes xxd hexdump od strings
ping traceroute dig nslookup host ifconfig ip ss netstat journalctl dmesg locate fd ncdu trash-list`)

// devTools run project code, or edit files in ways the command line alone does not reveal.
var devTools = strings.Fields(`make cmake cargo go npm npx yarn pnpm node python python3 ruby gem gcc g++ cc clang rustc javac java mvn gradle
code vim vi nano emacs gzip gunzip bzip2 xz zcat 7z zip source . sftp ffmpeg convert pandoc trash-put trash`)

// interpreters that execute whatever is fed to them.
var interpreters = map[string]bool{"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "fish": true, "python": true, "python3": true, "perl": true, "ruby": true, "node": true}

var rules map[string]rule

func init() {
	rules = map[string]rule{
		"rm": ruleRm, "unlink": fixed(R2, "Permanently removes a file."), "shred": ruleShred, "srm": ruleShred,
		"truncate": fixed(R2, "Cuts a file to a fixed size and can remove its contents."),
		"mv":       ruleMv, "cp": ruleCp, "ln": ruleLn,
		"mkdir": fixed(R1, "Creates folders. Easy to undo."), "touch": fixed(R1, "Creates or touches files. Easy to undo."),
		"rmdir": fixed(R1, "Removes empty folders only."),
		"dd":    ruleDd, "mkfs": fixed(R4, "Formats a disk or partition. All data on it is destroyed."),
		"mkswap":     fixed(R4, "Formats a disk or partition as swap. All data on it is destroyed."),
		"blkdiscard": fixed(R4, "Discards every block on a device. All data on it is destroyed."),
		"wipefs":     ruleWipefs, "fdisk": ruleFdisk, "sfdisk": ruleFdisk, "gdisk": ruleFdisk, "parted": ruleParted,
		"chmod": rulePerm, "chown": rulePerm, "chgrp": rulePerm,
		"kill": ruleKill, "pkill": rulePkill, "killall": rulePkill,
		"killall5": fixed(R4, "Kills every process on the machine."),
		"shutdown": rulePower, "reboot": rulePower, "halt": rulePower, "poweroff": rulePower, "telinit": rulePower,
		"systemctl": ruleSystemctl, "service": ruleService,
		"sed": ruleSed, "find": ruleFind, "tee": ruleTee,
		"git": ruleGit, "docker": ruleDocker, "podman": ruleDocker, "kubectl": ruleKubectl,
		"terraform": ruleTerraform, "tofu": ruleTerraform,
		"aws": ruleCloud, "gcloud": ruleCloud, "az": ruleCloud,
		"apt": rulePkg, "apt-get": rulePkg, "dpkg": rulePkg, "dnf": rulePkg, "yum": rulePkg, "pacman": rulePkg, "zypper": rulePkg, "snap": rulePkg,
		"pip": rulePkg, "pip3": rulePkg,
		"psql": ruleDB, "mysql": ruleDB, "mariadb": ruleDB, "sqlite3": ruleDB, "mongosh": ruleDB, "redis-cli": ruleDB,
		"dropdb": fixed(R3, "Deletes a whole database permanently."), "dropuser": fixed(R3, "Deletes a database user permanently."),
		"curl": ruleCurl, "wget": ruleCurl, "rsync": ruleRsync,
		"iptables": ruleFirewall, "ip6tables": ruleFirewall, "nft": ruleFirewall, "ufw": ruleFirewall,
		"crontab":  ruleCrontab,
		"userdel":  fixed(R3, "Deletes a user account, and with options its home folder."),
		"deluser":  fixed(R3, "Deletes a user account, and with options its home folder."),
		"groupdel": fixed(R3, "Deletes a group."),
		"rmmod":    fixed(R3, "Unloads a kernel module, which can crash drivers or the system."),
		"insmod":   fixed(R3, "Loads code into the kernel."), "modprobe": fixed(R2, "Loads or unloads kernel modules."),
		"umount": fixed(R2, "Detaches a filesystem. Programs using it may fail."), "swapoff": fixed(R2, "Turns swap off, which can run the machine out of memory."),
		"mount": fixed(R1, "Attaches a filesystem."), "sysctl": ruleSysctl,
		"passwd": fixed(R2, "Changes a password."), "chroot": fixed(R2, "Runs commands inside a different root folder."),
		"visudo": fixed(R2, "Edits who is allowed to use sudo."),
		"sh":     ruleShell, "bash": ruleShell, "zsh": ruleShell, "dash": ruleShell, "ksh": ruleShell, "fish": ruleShell, "eval": ruleShell,
		"ssh": fixed(R1, "Connects to another machine. Commands run there are checked separately."), "scp": fixed(R1, "Copies files to or from another machine. Existing files may be overwritten."),
		"tar": ruleTar, "unzip": ruleUnzip, "npm": ruleNpm,
	}
}

// fixed returns a rule that always gives the same answer.
func fixed(l Level, reason string) rule {
	return func(c shellparse.Command, _ flags) []Finding { return one(fnd(l, c, reason)) }
}

// ---- deleting and overwriting files ----

func ruleRm(c shellparse.Command, f flags) []Finding {
	recursive := f.has('r', "recursive") || f.has('R', "")
	var out []Finding
	if f.hasAny("--no-preserve-root") {
		out = append(out, fnd(R4, c, "Turns off the safety check that stops `rm` from deleting the whole filesystem."))
	}
	for _, p := range f.pos {
		ok, what := critical(p)
		switch {
		case ok && recursive:
			out = append(out, fnd(R4, c, "Recursively deletes "+what+". This would destroy the system or all your data."))
		case ok:
			out = append(out, fnd(R3, c, "Deletes "+what+" permanently."))
		}
	}
	if maxLevel(out) >= R3 {
		return out
	}
	trash := "trash-put " + targets(f)
	switch {
	case recursive && anyWholeCwd(f.pos):
		return one(fnd(R3, c, "Recursively deletes everything in the current directory. Deleted files cannot be recovered.").
			alt(trash, "moves them to the Trash so they can be restored"))
	case recursive:
		return one(fnd(R3, c, "Recursively and permanently deletes whole folders. There is no recycle bin.").
			alt(trash, "moves them to the Trash so they can be restored"))
	case c.ViaXargs || c.ViaFind:
		return one(fnd(R2, c, "Deletes whatever files are passed in, and you cannot see the list first."))
	case len(f.pos) == 0:
		return one(fnd(R1, c, "No files given, so nothing will be deleted."))
	}
	return one(fnd(R2, c, "Permanently deletes files. There is no recycle bin.").alt(trash, "moves them to the Trash so they can be restored"))
}

func anyWholeCwd(ps []string) bool {
	for _, p := range ps {
		if wholeCwd(p) {
			return true
		}
	}
	return false
}

func targets(f flags) string {
	q := make([]string, len(f.pos))
	for i, p := range f.pos {
		q[i] = shq(p)
	}
	return strings.Join(q, " ")
}

func ruleShred(c shellparse.Command, f flags) []Finding {
	for _, p := range f.pos {
		if isBlockDevice(p) {
			return one(fnd(R4, c, "Overwrites a whole disk with random data. Everything on it is destroyed."))
		}
	}
	return one(fnd(R3, c, "Overwrites files so they can never be recovered, even with recovery tools."))
}

func ruleMv(c shellparse.Command, f flags) []Finding {
	var out []Finding
	for i, p := range f.pos {
		if ok, what := critical(p); ok && i < len(f.pos)-1 {
			out = append(out, fnd(R3, c, "Moves "+what+" away, which can break the system or lose access to your files."))
		}
	}
	if n := len(f.pos); n > 1 && f.pos[n-1] == "/dev/null" {
		out = append(out, fnd(R3, c, "Moving files to /dev/null destroys them."))
	}
	if len(out) > 0 {
		return out
	}
	if f.has('f', "force") {
		return one(fnd(R2, c, "Overwrites existing files at the destination without asking.").alt("mv -i "+targets(f), "asks before overwriting"))
	}
	return one(fnd(R1, c, "Moves files. Undo is moving them back, unless one overwrites an existing file."))
}

func ruleCp(c shellparse.Command, f flags) []Finding {
	if f.has('f', "force") {
		return one(fnd(R2, c, "Overwrites existing files at the destination without asking.").alt("cp -i "+targets(f), "asks before overwriting"))
	}
	return one(fnd(R1, c, "Copies files. An existing file at the destination is overwritten."))
}

func ruleLn(c shellparse.Command, f flags) []Finding {
	if f.has('f', "force") {
		return one(fnd(R2, c, "Replaces an existing file or link."))
	}
	return one(fnd(R1, c, "Creates a link."))
}

// ---- disks ----

func ruleDd(c shellparse.Command, f flags) []Finding {
	for _, a := range f.all {
		if v, ok := strings.CutPrefix(a, "of="); ok {
			if isBlockDevice(v) {
				return one(fnd(R4, c, "Writes raw data straight onto a disk. Everything on "+v+" will be destroyed.").
					alt("lsblk", "check which disk is which before writing anything"))
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

// ---- permissions ----

func rulePerm(c shellparse.Command, f flags) []Finding {
	recursive := f.has('R', "recursive")
	var out []Finding
	for _, p := range f.pos {
		if ok, what := critical(p); ok && recursive {
			out = append(out, fnd(R4, c, "Changes permissions or owners across "+what+". This can break logins, sudo and the whole system.").
				alt("chmod -R u+rwX,go-w "+shq(p), "adds only what you need, and does not open files to everyone"))
		}
	}
	if len(out) > 0 {
		return out
	}
	insecure := c.Name == "chmod" && f.hasAny("777", "666", "a+rwx", "ugo+rwx")
	switch {
	case recursive && insecure:
		return one(fnd(R2, c, "Gives everyone full access to every file in the folder. This is a security hole.").
			alt("chmod -R u+rwX,go-w "+targetsAfterMode(f), "grants access to you only"))
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

// ---- processes and power ----

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
			alt("kill "+strings.Join(f.pos, " "), "asks the program to quit politely first"))
	}
	return one(fnd(R1, c, "Asks a process to stop."))
}

func rulePkill(c shellparse.Command, f flags) []Finding {
	alt := "pgrep -a " + strings.Join(f.pos, " ")
	return one(fnd(R2, c, "Stops every process that matches a name or pattern, which may include more than you expect.").
		alt(alt, "lists what would match, without stopping anything"))
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

// ---- editing in place ----

func ruleSed(c shellparse.Command, f flags) []Finding {
	if f.has('i', "in-place") {
		return one(fnd(R2, c, "Edits files in place. The original text is not kept.").alt(strings.Replace(c.String(), "sed -i", "sed -i.bak", 1), "keeps a .bak copy of each file"))
	}
	return one(fnd(R0, c, "Only prints the edited text; files are not changed."))
}

func ruleTee(c shellparse.Command, f flags) []Finding {
	if f.has('a', "append") {
		return one(fnd(R1, c, "Appends to files."))
	}
	return one(fnd(R2, c, "Overwrites the files it is given."))
}

func ruleFind(c shellparse.Command, f flags) []Finding {
	if f.hasAny("-delete") {
		for _, p := range f.pos {
			if ok, what := critical(p); ok {
				return one(fnd(R4, c, "Deletes every match under "+what+"."))
			}
		}
		return one(fnd(R3, c, "Deletes every file that matches. You cannot see the list first.").
			alt(strings.Replace(c.String(), "-delete", "-print", 1), "only lists what would be deleted"))
	}
	return one(fnd(R0, c, "Only searches. Any -exec command is checked separately."))
}

// ---- developer and infrastructure tools ----

func ruleGit(c shellparse.Command, f flags) []Finding {
	switch f.sub() {
	case "push":
		switch {
		case f.hasAny("--force-with-lease") || f.long["force-with-lease"] != "" || f.has(0, "force-with-lease"):
			return one(fnd(R2, c, "Rewrites history on the remote, but refuses if someone else pushed in the meantime."))
		case f.has('f', "force") || f.hasAny("--delete", "-d") || anyPrefix(f.pos, ":"):
			return one(fnd(R3, c, "Overwrites or deletes history on the shared remote. Other people's work can be lost.").
				alt(strings.Replace(c.String(), "--force", "--force-with-lease", 1), "refuses if someone else pushed in the meantime"))
		}
		return one(fnd(R2, c, "Publishes commits to a shared remote. Hard to take back once others pull them."))
	case "reset":
		if f.hasAny("--hard") {
			return one(fnd(R2, c, "Throws away all uncommitted changes. Committed work stays recoverable through the reflog.").
				alt("git stash", "saves your changes first so they can be brought back"))
		}
		return one(fnd(R1, c, "Moves the branch pointer. Files are kept."))
	case "clean":
		if f.has('n', "dry-run") {
			return one(fnd(R0, c, "Only lists the untracked files it would remove."))
		}
		if f.has('f', "force") {
			return one(fnd(R3, c, "Deletes untracked files that git does not track, so they cannot be restored.").
				alt("git clean -n"+strings.TrimPrefix(c.String(), "git clean"), "a dry run that lists what would be deleted"))
		}
		return one(fnd(R1, c, "Refuses to delete without -f."))
	case "checkout", "restore", "switch":
		if f.hasAny(".") || f.hasAny("--", "-f", "--force", "--discard-changes") && f.hasAny(".") {
			return one(fnd(R2, c, "Discards uncommitted changes in the working folder."))
		}
		return one(fnd(R1, c, "Switches branches or restores files."))
	case "branch":
		if f.hasAny("-D") || (f.has('d', "delete") && f.has('f', "force")) {
			return one(fnd(R2, c, "Deletes a branch even if it is not merged. Recoverable through the reflog."))
		}
		return one(fnd(R0, c, "Lists or creates branches."))
	case "rebase", "filter-branch", "filter-repo", "commit-tree":
		return one(fnd(R2, c, "Rewrites commit history."))
	case "gc", "reflog", "prune":
		if f.hasAny("--prune=now", "--expire=now", "--expire-unreachable=now") {
			return one(fnd(R3, c, "Permanently erases the safety net (reflog and unreachable commits) used to recover lost work."))
		}
		return one(fnd(R1, c, "Housekeeping."))
	case "stash":
		if len(f.pos) > 1 && (f.pos[1] == "drop" || f.pos[1] == "clear") {
			return one(fnd(R2, c, "Deletes saved stashes."))
		}
		return one(fnd(R1, c, "Saves or restores changes."))
	case "status", "log", "diff", "show", "fetch", "blame", "ls-files", "describe", "remote", "config", "rev-parse", "grep", "shortlog", "ls-remote", "":
		return one(fnd(R0, c, "Only reads repository information."))
	}
	return one(fnd(R1, c, "Changes the local repository. Reversible through git history."))
}

func anyPrefix(ss []string, p string) bool {
	for _, s := range ss {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func ruleDocker(c shellparse.Command, f flags) []Finding {
	switch f.sub() {
	case "ps", "images", "logs", "inspect", "info", "version", "stats", "top", "history", "ls", "search", "diff", "port":
		return one(fnd(R0, c, "Only reads container information."))
	case "stop", "kill", "restart", "pause":
		return one(fnd(R2, c, "Stops running containers and the services in them."))
	case "rm", "rmi":
		return one(fnd(R2, c, "Removes containers or images. Data inside containers is lost."))
	case "run":
		if f.hasAny("--privileged") || strings.Contains(f.joined(), "-v /:/") || strings.Contains(f.joined(), "--volume /:/") {
			return one(fnd(R3, c, "Gives the container root-level access to the host machine."))
		}
		return one(fnd(R1, c, "Starts a container."))
	case "system", "volume", "network", "container", "image", "builder":
		if len(f.pos) > 1 && (f.pos[1] == "prune" || f.pos[1] == "rm") {
			if c.Name != "" && (f.pos[0] == "volume" || f.hasAny("--volumes") || f.hasAny("-a", "-af", "--all")) {
				return one(fnd(R3, c, "Permanently deletes unused containers, images or volumes. Volume data is lost."))
			}
			return one(fnd(R2, c, "Removes unused Docker objects."))
		}
	}
	return one(fnd(R1, c, "Runs a container command."))
}

func ruleKubectl(c shellparse.Command, f flags) []Finding {
	switch f.sub() {
	case "get", "describe", "logs", "diff", "top", "explain", "version", "config", "api-resources", "events", "cluster-info":
		return one(fnd(R0, c, "Only reads cluster information."))
	case "delete":
		if f.hasAny("--all", "-A", "--all-namespaces") || (len(f.pos) > 1 && (f.pos[1] == "namespace" || f.pos[1] == "ns" || f.pos[1] == "namespaces")) {
			return one(fnd(R4, c, "Deletes a whole namespace or every object of a kind. This can take down production services."))
		}
		return one(fnd(R3, c, "Deletes cluster objects. Running services stop and their data may be lost.").
			alt(strings.Replace(c.String(), "delete", "delete --dry-run=server", 1), "asks the server what would be deleted, without deleting"))
	case "drain":
		return one(fnd(R3, c, "Evicts every workload from a node."))
	case "apply", "create", "scale", "patch", "replace", "rollout", "cordon", "uncordon", "taint", "label", "annotate", "edit":
		return one(fnd(R2, c, "Changes live cluster objects.").alt(c.String()+" --dry-run=server", "previews the change without applying it"))
	}
	return one(fnd(R2, c, "Changes live cluster objects."))
}

func ruleTerraform(c shellparse.Command, f flags) []Finding {
	switch f.sub() {
	case "plan", "validate", "fmt", "show", "output", "version", "init", "state", "providers", "graph":
		return one(fnd(R0, c, "Only reads or previews infrastructure."))
	case "destroy":
		return one(fnd(R3, c, "Destroys real infrastructure and everything running on it.").alt(strings.Replace(c.String(), "destroy", "plan -destroy", 1), "shows what would be destroyed"))
	case "apply":
		if f.hasAny("-auto-approve") {
			return one(fnd(R3, c, "Changes real infrastructure without showing you the plan first.").alt("terraform plan", "shows the plan first"))
		}
		return one(fnd(R2, c, "Changes real infrastructure after showing a plan."))
	}
	return one(fnd(R2, c, "Changes infrastructure state."))
}

var cloudDestroy = []string{"delete", "terminate", "destroy", "remove", "rm", "purge", "drop", "deregister", "detach", "release", "revoke"}
var cloudRead = []string{"describe", "list", "get", "ls", "show", "view", "info", "status", "whoami", "version"}

func ruleCloud(c shellparse.Command, f flags) []Finding {
	for _, p := range f.pos {
		low := strings.ToLower(p)
		for _, d := range cloudDestroy {
			if low == d || strings.HasPrefix(low, d+"-") {
				return one(fnd(R3, c, "Deletes cloud resources. This usually cannot be undone and can cost data or uptime."))
			}
		}
	}
	if f.hasAny("--delete") {
		return one(fnd(R3, c, "Deletes files at the destination that are missing from the source."))
	}
	for _, p := range f.pos {
		low := strings.ToLower(p)
		for _, d := range cloudRead {
			if low == d || strings.HasPrefix(low, d+"-") {
				return one(fnd(R0, c, "Only reads cloud information."))
			}
		}
	}
	return one(fnd(R2, c, "Changes cloud resources."))
}

func rulePkg(c shellparse.Command, f flags) []Finding {
	if f.hasAny("-n", "--dry-run", "--simulate", "-s", "--just-print") {
		return one(fnd(R0, c, "Dry run: shows what would change without changing it."))
	}
	switch f.sub() {
	case "remove", "purge", "autoremove", "erase", "uninstall", "-R", "-Rs", "-Rns", "-r", "-P":
		return one(fnd(R2, c, "Removes software, which can also remove things that depend on it.").alt(c.String()+" --dry-run", "shows what would be removed"))
	case "list", "search", "show", "info", "policy", "-l", "-L", "-S", "-s", "check", "freeze":
		return one(fnd(R0, c, "Only reads package information."))
	}
	return one(fnd(R1, c, "Installs or updates software."))
}

func ruleNpm(c shellparse.Command, f flags) []Finding {
	switch f.sub() {
	case "uninstall", "remove", "rm", "unpublish", "deprecate":
		return one(fnd(R2, c, "Removes or unpublishes packages."))
	}
	return one(fnd(R1, c, "Runs package or project scripts, whose effects depend on the project."))
}

var (
	sqlDropRe   = `(?i)\bdrop\s+(database|schema|table)\b|\btruncate\b`
	sqlDeleteRe = `(?i)\b(delete\s+from|update\s+\w+\s+set)\b`
)

func ruleDB(c shellparse.Command, f flags) []Finding {
	text := f.joined()
	up := strings.ToUpper(text)
	switch {
	case c.Name == "redis-cli" && (strings.Contains(up, "FLUSHALL") || strings.Contains(up, "FLUSHDB")):
		return one(fnd(R3, c, "Erases every key in the database."))
	case matches(sqlDropRe, text):
		return one(fnd(R3, c, "Drops or empties tables or databases. The data is gone unless there is a backup."))
	case matches(sqlDeleteRe, text) && !strings.Contains(up, " WHERE "):
		return one(fnd(R3, c, "Changes or deletes EVERY row, because there is no WHERE condition."))
	case matches(sqlDeleteRe, text):
		return one(fnd(R2, c, "Changes or deletes rows in a database."))
	case strings.Contains(up, "INSERT") || strings.Contains(up, "ALTER") || strings.Contains(up, "CREATE"):
		return one(fnd(R2, c, "Changes database contents or structure."))
	case strings.Contains(up, "SELECT") || strings.Contains(up, "\\DT") || strings.Contains(up, "SHOW"):
		return one(fnd(R0, c, "Only reads data."))
	}
	return one(fnd(R2, c, "Opens a database session. What you type inside it is not checked."))
}

func ruleCurl(c shellparse.Command, f flags) []Finding {
	if c.Name == "curl" && (f.hasAny("-X", "--request") && (f.hasAny("POST", "PUT", "DELETE", "PATCH") || strings.Contains(f.joined(), "POST") || strings.Contains(f.joined(), "DELETE")) ||
		f.hasAny("-d", "--data", "-F", "--form", "-T", "--upload-file", "--data-raw", "--data-binary")) {
		return one(fnd(R2, c, "Sends data to, or changes something on, a remote server. That cannot be undone from here."))
	}
	return one(fnd(R1, c, "Downloads from the network."))
}

func ruleRsync(c shellparse.Command, f flags) []Finding {
	for _, a := range f.all {
		if strings.HasPrefix(a, "--delete") || a == "--remove-source-files" {
			return one(fnd(R3, c, "Deletes files at the destination (or source) that do not match. A wrong path wipes a folder.").
				alt(c.String()+" --dry-run", "lists what would change, without changing it"))
		}
	}
	return one(fnd(R1, c, "Copies and syncs files."))
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
		return one(fnd(R3, c, "Deletes ALL scheduled jobs for the user, with no confirmation.").alt("crontab -l > ~/crontab.bak", "saves a copy first"))
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

func ruleTar(c shellparse.Command, f flags) []Finding {
	if f.hasAny("--remove-files") {
		return one(fnd(R2, c, "Deletes the original files after adding them to the archive."))
	}
	return one(fnd(R1, c, "Creates or extracts an archive. Extracting can overwrite files."))
}

func ruleUnzip(c shellparse.Command, f flags) []Finding {
	if f.has('o', "") {
		return one(fnd(R2, c, "Extracts and overwrites existing files without asking."))
	}
	return one(fnd(R1, c, "Extracts an archive."))
}
