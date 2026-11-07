//go:build linux

package platform

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Probe inspects the machine. It changes nothing: the namespace check mounts a
// tmpfs on a temporary directory inside a throwaway namespace.
func Probe() Report {
	r := Report{
		Shells: lookPath("bash", "zsh", "fish"),
	}
	r.Kernel = readTrim("/proc/sys/kernel/osrelease")
	r.AppArmorRestrictsUserNS = readTrim("/proc/sys/kernel/apparmor_restrict_unprivileged_userns") == "1"
	r.Overlay = strings.Contains(readFile("/proc/filesystems"), "overlay")
	r.LSMs = parseLSM(readFile("/sys/kernel/security/lsm"))
	r.Landlock = has(r.LSMs, "landlock")
	r.Seccomp = strings.Contains(readFile("/proc/self/status"), "Seccomp:")
	_, err := os.Stat("/sys/fs/cgroup/cgroup.controllers")
	r.CgroupV2 = err == nil
	r.RootFS = rootFSType(readFile("/proc/self/mountinfo"))
	if p, err := exec.LookPath("bwrap"); err == nil {
		r.Bwrap = p
	}
	r.UserNS, r.UserNSErr = probeUserNS()
	return r
}

// probeUserNS makes a user+mount namespace and mounts a tmpfs inside it. Making
// the namespace is not enough: on Ubuntu 24.04+ AppArmor lets an unconfined
// program create one but strips its capabilities, so every mount then fails.
func probeUserNS() (bool, string) {
	dir, err := os.MkdirTemp("", "ss-probe-")
	if err != nil {
		return false, err.Error()
	}
	defer os.Remove(dir)
	cmd := exec.Command("/bin/mount", "-t", "tmpfs", "tmpfs", dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return false, strings.SplitN(msg, "\n", 2)[0]
	}
	return true, ""
}

func readFile(p string) string {
	b, _ := os.ReadFile(p)
	return string(b)
}

func readTrim(p string) string { return strings.TrimSpace(readFile(p)) }
