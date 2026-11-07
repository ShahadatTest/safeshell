package overlayfs

import (
	"slices"
	"testing"
)

const mountinfo = `22 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
23 22 0:5 / /dev rw,nosuid shared:2 - devtmpfs udev rw
24 22 0:21 / /proc rw shared:3 - proc proc rw
25 22 0:22 / /sys rw shared:4 - sysfs sysfs rw
26 22 0:23 / /run rw shared:5 - tmpfs tmpfs rw
27 26 0:24 / /run/lock rw shared:6 - tmpfs tmpfs rw
28 22 8:3 / /home rw shared:7 - ext4 /dev/sda3 rw
29 28 8:4 / /home/me/My\040Disk rw shared:8 - btrfs /dev/sdb1 rw
30 22 0:25 / /tmp rw shared:9 - tmpfs tmpfs rw
31 22 7:1 / /snap/core/1 ro shared:10 - squashfs /dev/loop1 ro
32 22 0:26 / /sys/fs/cgroup rw shared:11 - cgroup2 cgroup2 rw
33 22 0:27 / /mnt/share rw shared:12 - nfs4 host:/x rw
garbage line`

func TestMountPoints(t *testing.T) {
	got := mountPoints(mountinfo)
	want := []string{"/home", "/tmp", "/home/me/My Disk"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestMountPointsEmpty(t *testing.T) {
	if got := mountPoints(""); len(got) != 0 {
		t.Errorf("got %q", got)
	}
}

func TestLayerMounts(t *testing.T) {
	got := layerMounts([]string{"usr", "etc", "home", "tmp"}, mountinfo)
	want := []string{"/etc", "/home", "/tmp", "/usr", "/home/me/My Disk"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if len(layerMounts(nil, "")) != 0 {
		t.Error("nothing in, nothing out")
	}
}

func TestIsSpecialTop(t *testing.T) {
	for n, want := range map[string]bool{"proc": true, "sys": true, "dev": true, "run": true, "usr": false, "tmp": false, "": false} {
		if isSpecialTop(n) != want {
			t.Errorf("isSpecialTop(%q) != %v", n, want)
		}
	}
}

func TestParentsComeBeforeChildren(t *testing.T) {
	mps := mountPoints(mountinfo)
	for i, m := range mps {
		for j, p := range mps {
			if m != p && len(m) > len(p) && m[:len(p)+1] == p+"/" && j > i {
				t.Errorf("%s must be mounted before %s", p, m)
			}
		}
	}
}

func TestUnescape(t *testing.T) {
	for in, want := range map[string]string{`a\040b`: "a b", `plain`: "plain", `a\011b`: "a\tb", `bad\9zz`: `bad\9zz`, `end\04`: `end\04`} {
		if got := unescape(in); got != want {
			t.Errorf("unescape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlan(t *testing.T) {
	ls := plan([]string{"/", "/home"}, "/tmp/s")
	if ls[0].Upper != "/tmp/s/layers/0/upper" || ls[1].Work != "/tmp/s/layers/1/work" || ls[1].Mount != "/home" {
		t.Errorf("plan = %+v", ls)
	}
	if itoa(0) != "0" || itoa(12) != "12" || itoa(305) != "305" {
		t.Error("itoa wrong")
	}
}
