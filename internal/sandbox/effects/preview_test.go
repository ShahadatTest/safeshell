package effects

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/samarthsinh2660/safeshell/internal/domain"
	"github.com/samarthsinh2660/safeshell/internal/shellparse"
)

func cmds(t *testing.T, line string) []shellparse.Command {
	t.Helper()
	res, err := shellparse.Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	var out []shellparse.Command
	for _, p := range res.Pipelines {
		out = append(out, p.Cmds...)
	}
	return out
}

func aptOutput(names ...string) string {
	var sb strings.Builder
	sb.WriteString("Reading package lists...\nThe following packages will be REMOVED:\n")
	for _, n := range names {
		fmt.Fprintf(&sb, "Remv %s [1.0-1]\n", n)
	}
	return sb.String()
}

func TestPackagePreviewUsesAptsOwnSimulation(t *testing.T) {
	var gotName string
	var gotArgs []string
	run := func(name string, args ...string) (string, error) {
		gotName, gotArgs = name, args
		return aptOutput("htop"), nil
	}
	es := PreviewPackages(run, cmds(t, "sudo apt remove -y htop"))
	if gotName != "apt-get" || gotArgs[0] != "-s" || strings.Contains(strings.Join(gotArgs, " "), "-y") || !strings.Contains(strings.Join(gotArgs, " "), "remove htop") {
		t.Errorf("must simulate and never assume yes: %s %v", gotName, gotArgs)
	}
	if len(es) != 1 || es[0].Evidence != domain.Previewed || es[0].MinLevel != domain.R2 || es[0].Kind != domain.EffectPackages || !strings.Contains(es[0].Summary, "1 package") {
		t.Errorf("%+v", es)
	}
}

func TestPackagePreviewRaisesForEssentialAndBulkRemovals(t *testing.T) {
	ess := PreviewPackages(func(string, ...string) (string, error) { return aptOutput("gnome-shell", "libfoo"), nil }, cmds(t, "apt-get purge gnome-shell"))
	if len(ess) != 1 || ess[0].MinLevel != domain.R4 || !strings.Contains(ess[0].Summary, "gnome-shell") {
		t.Errorf("%+v", ess)
	}
	var many []string
	for i := 0; i < 30; i++ {
		many = append(many, fmt.Sprintf("lib%d", i))
	}
	bulk := PreviewPackages(func(string, ...string) (string, error) { return aptOutput(many...), nil }, cmds(t, "apt autoremove"))
	if len(bulk) != 1 || bulk[0].MinLevel != domain.R3 || len(bulk[0].Details) != 13 {
		t.Errorf("%+v", bulk)
	}
}

func TestPackagePreviewIgnoresOtherCommandsAndMissingApt(t *testing.T) {
	never := func(string, ...string) (string, error) { t.Fatal("must not run"); return "", nil }
	if es := PreviewPackages(never, cmds(t, "apt install htop; ls; apt-get update")); len(es) != 0 {
		t.Errorf("installs are not previewed here: %+v", es)
	}
	if es := PreviewPackages(func(string, ...string) (string, error) { return "", errors.New("not found") }, cmds(t, "apt remove x")); len(es) != 0 {
		t.Errorf("no apt, no effect: %+v", es)
	}
}

func fakeSystemctl(deps string, active map[string]bool) Runner {
	return func(name string, args ...string) (string, error) {
		if args[0] == "list-dependencies" {
			return deps, nil
		}
		var lines []string
		for _, u := range args[1:] {
			if active[u] {
				lines = append(lines, "active")
			} else {
				lines = append(lines, "inactive")
			}
		}
		return strings.Join(lines, "\n"), nil
	}
}

func TestServicePreviewListsOnlyRunningDependents(t *testing.T) {
	run := fakeSystemctl("postgresql.service\n  app.service\n  worker.service\n  old.service\n", map[string]bool{"app.service": true, "worker.service": true})
	es := InspectServices(run, cmds(t, "sudo systemctl stop postgresql"))
	if len(es) != 1 {
		t.Fatalf("%+v", es)
	}
	e := es[0]
	if e.Evidence != domain.Inspected || e.Kind != domain.EffectServices || !strings.Contains(e.Summary, "postgresql.service") || !strings.Contains(e.Summary, "2 running") {
		t.Errorf("%+v", e)
	}
	if got := strings.Join(e.Details, "|"); !strings.Contains(got, "app.service is running") || strings.Contains(got, "old.service") {
		t.Errorf("details %q", got)
	}
}

func TestStoppingACriticalServiceRaisesToIrreversible(t *testing.T) {
	for _, line := range []string{"systemctl stop sshd", "systemctl disable NetworkManager", "systemctl mask display-manager.service"} {
		es := InspectServices(fakeSystemctl("", nil), cmds(t, line))
		if len(es) != 1 || es[0].MinLevel != domain.R3 {
			t.Errorf("%s: %+v", line, es)
		}
	}
	if es := InspectServices(fakeSystemctl("", nil), cmds(t, "systemctl restart ssh")); es[0].MinLevel != domain.R2 {
		t.Errorf("a restart is brief: %+v", es)
	}
}

func TestManyDependentsRaiseAndReadsAreIgnored(t *testing.T) {
	deps := "base.service\n"
	active := map[string]bool{}
	for i := 0; i < 12; i++ {
		u := fmt.Sprintf("svc%d.service", i)
		deps += "  " + u + "\n"
		active[u] = true
	}
	es := InspectServices(fakeSystemctl(deps, active), cmds(t, "systemctl stop base"))
	if len(es) != 1 || es[0].MinLevel != domain.R3 {
		t.Errorf("%+v", es)
	}
	never := func(string, ...string) (string, error) { t.Fatal("must not run"); return "", nil }
	if es := InspectServices(never, cmds(t, "systemctl status nginx; systemctl start nginx; ls")); len(es) != 0 {
		t.Errorf("%+v", es)
	}
}
