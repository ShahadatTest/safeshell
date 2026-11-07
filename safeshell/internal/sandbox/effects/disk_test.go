package effects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"safeshell/internal/domain"
	"safeshell/internal/shellparse"
)

// fakeSys builds a /sys and /dev tree with disk sda (partitions sda1 and sda2)
// and an unmounted disk sdb with one partition.
func fakeSys(t *testing.T) SysFS {
	t.Helper()
	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	blockDev := func(disk, major string, partNames ...string) {
		mk("sys/devices/pci0/block/"+disk+"/size", "1000215216\n") // 476.9 GiB
		mk("sys/devices/pci0/block/"+disk+"/dev", major+":0\n")
		os.MkdirAll(filepath.Join(root, "sys/class/block"), 0o755)
		os.Symlink("../../devices/pci0/block/"+disk, filepath.Join(root, "sys/class/block", disk))
		os.WriteFile(filepath.Join(root, "dev-"+disk), nil, 0o644)
		mk("dev/"+disk, "")
		for i, p := range partNames {
			mk("sys/devices/pci0/block/"+disk+"/"+p+"/partition", "1\n")
			mk("sys/devices/pci0/block/"+disk+"/"+p+"/size", "2097152\n") // 1 GiB
			mk("sys/devices/pci0/block/"+disk+"/"+p+"/dev", major+":"+string(rune('1'+i))+"\n")
			os.Symlink("../../devices/pci0/block/"+disk+"/"+p, filepath.Join(root, "sys/class/block", p))
			mk("dev/"+p, "")
		}
	}
	blockDev("sda", "8", "sda1", "sda2")
	blockDev("sdb", "8", "sdb1")
	// make sdb a different major so its numbers do not collide with sda's
	os.WriteFile(filepath.Join(root, "sys/devices/pci0/block/sdb/dev"), []byte("9:0\n"), 0o644)
	os.WriteFile(filepath.Join(root, "sys/devices/pci0/block/sdb/sdb1/dev"), []byte("9:1\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "dev/disk/by-id"), 0o755)
	os.Symlink("../../sda", filepath.Join(root, "dev/disk/by-id/ata-FAKE"))

	mi := `22 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
23 22 8:1 / /boot/efi rw shared:2 - vfat /dev/sda1 rw
24 22 0:5 / /dev rw shared:3 - devtmpfs udev rw`
	return SysFS{Dev: filepath.Join(root, "dev"), Sys: filepath.Join(root, "sys"), Mountinfo: mi}
}

func TestDdOntoTheSystemDisk(t *testing.T) {
	fs := fakeSys(t)
	fx := InspectDisks(fs, []shellparse.Command{{Name: "dd", Args: []string{"if=/dev/zero", "of=/dev/sda", "bs=1M"}}})
	if len(fx) != 1 {
		t.Fatalf("%+v", fx)
	}
	e := fx[0]
	if e.Kind != domain.EffectDisk || e.Evidence != domain.Inspected || e.MinLevel != domain.R4 {
		t.Errorf("%+v", e)
	}
	for _, want := range []string{"/dev/sda", "a disk of 476.9 GiB", "holds the running system"} {
		if !strings.Contains(e.Summary, want) {
			t.Errorf("summary missing %q: %s", want, e.Summary)
		}
	}
	joined := strings.Join(e.Details, "; ")
	if !strings.Contains(joined, "2 partition(s): sda1, sda2") || !strings.Contains(joined, "mounted at /, /boot/efi") {
		t.Errorf("details: %s", joined)
	}
}

func TestPartitionAndUnmountedDisk(t *testing.T) {
	fs := fakeSys(t)
	fx := InspectDisks(fs, []shellparse.Command{{Name: "mkfs.ext4", Args: []string{"/dev/sda2"}}})
	if len(fx) != 1 || !strings.Contains(fx[0].Summary, "a partition of 1.0 GiB") || !strings.Contains(fx[0].Summary, "holds the running system") {
		t.Errorf("mounted system partition: %+v", fx)
	}
	fx = InspectDisks(fs, []shellparse.Command{{Name: "wipefs", Args: []string{"-a", "/dev/sdb"}}})
	if len(fx) != 1 || strings.Contains(fx[0].Summary, "running system") || !strings.Contains(strings.Join(fx[0].Details, ";"), "not mounted") {
		t.Errorf("an unmounted USB disk: %+v", fx)
	}
}

func TestSymlinkedDeviceNamesAreResolved(t *testing.T) {
	fs := fakeSys(t)
	fx := InspectDisks(fs, []shellparse.Command{{Name: "shred", Args: []string{"/dev/disk/by-id/ata-FAKE"}}})
	if len(fx) != 1 || !strings.Contains(fx[0].Summary, "476.9 GiB") {
		t.Errorf("%+v", fx)
	}
}

func TestRedirectToADevice(t *testing.T) {
	fs := fakeSys(t)
	c := shellparse.Command{Name: "cat", Args: []string{"image.iso"}, Redirs: []shellparse.Redirect{{Op: ">", Target: "/dev/sdb"}}}
	if fx := InspectDisks(fs, []shellparse.Command{c}); len(fx) != 1 {
		t.Errorf("%+v", fx)
	}
}

func TestUnknownDeviceIsEstimated(t *testing.T) {
	fs := fakeSys(t)
	fx := InspectDisks(fs, []shellparse.Command{{Name: "dd", Args: []string{"of=/dev/sdz"}}})
	if len(fx) != 1 || fx[0].Evidence != domain.Estimated || fx[0].MinLevel != domain.R4 {
		t.Errorf("%+v", fx)
	}
}

func TestIgnoresNonDisksAndNonWriters(t *testing.T) {
	fs := fakeSys(t)
	for _, c := range []shellparse.Command{
		{Name: "dd", Args: []string{"of=/dev/null"}},
		{Name: "cat", Args: []string{"/dev/sda"}}, // reading is fine
		{Name: "ls", Args: []string{"/dev/sda"}},
		{Name: "echo", Redirs: []shellparse.Redirect{{Op: ">", Target: "/dev/null"}}},
		{Name: "dd", Args: []string{"of=/tmp/file"}},
	} {
		if fx := InspectDisks(fs, []shellparse.Command{c}); len(fx) != 0 {
			t.Errorf("%v should not be reported: %+v", c, fx)
		}
	}
}

func TestSameDeviceIsReportedOnce(t *testing.T) {
	fs := fakeSys(t)
	cs := []shellparse.Command{
		{Name: "wipefs", Args: []string{"-a", "/dev/sdb"}},
		{Name: "dd", Args: []string{"of=/dev/sdb"}},
	}
	if fx := InspectDisks(fs, cs); len(fx) != 1 {
		t.Errorf("%+v", fx)
	}
}

func TestMountpointsAreSortedAndDeduplicated(t *testing.T) {
	mi := "1 0 8:1 / /b rw - ext4 x rw\n2 0 8:1 / /a rw - ext4 x rw\n3 0 8:1 / /a rw - ext4 x rw\n4 0 9:9 / /z rw - ext4 x rw"
	got := mountpoints(mi, map[string]bool{"8:1": true})
	if strings.Join(got, ",") != "/a,/b" {
		t.Errorf("got %v", got)
	}
}
