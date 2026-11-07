package shellparse

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// names lists every command found, pipeline by pipeline, as "a|b" strings.
func names(r *Result) []string {
	var out []string
	for _, p := range r.Pipelines {
		var ns []string
		for _, c := range p.Cmds {
			ns = append(ns, c.Name)
		}
		out = append(out, strings.Join(ns, "|"))
	}
	return out
}

func find(r *Result, name string) *Command {
	for _, p := range r.Pipelines {
		for i := range p.Cmds {
			if p.Cmds[i].Name == name {
				return &p.Cmds[i]
			}
		}
	}
	return nil
}

func mustParse(t *testing.T, line string) *Result {
	t.Helper()
	r, err := Parse(line)
	if err != nil {
		t.Fatalf("Parse(%q): %v", line, err)
	}
	return r
}

func TestStructure(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"ls -la", []string{"ls"}},
		{"cat f | grep x | wc -l", []string{"cat|grep|wc"}},
		{"make && make install || echo failed", []string{"make", "make", "echo"}},
		{"(cd /tmp && rm x)", []string{"cd", "rm"}},
		{"for f in *; do rm $f; done", []string{"rm"}},
		{"if true; then rm x; fi", []string{"true", "rm"}},
		{"echo $(rm x)", []string{"rm", "echo"}},
		{"bash -c 'rm -rf /'", []string{"rm", "bash"}},
		{`eval "rm x"`, []string{"rm", "eval"}},
		{"ssh host 'rm -rf /srv'", []string{"rm", "ssh"}},
		{"", nil},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := names(mustParse(t, tt.line))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("names = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWrappersAreStripped(t *testing.T) {
	tests := []struct {
		line     string
		name     string
		sudo     bool
		xargs    bool
		wantArgs []string
	}{
		{"sudo rm -rf /tmp/x", "rm", true, false, []string{"-rf", "/tmp/x"}},
		{"sudo -u bob rm x", "rm", true, false, []string{"x"}},
		{"doas rm x", "rm", true, false, []string{"x"}},
		{"env FOO=1 BAR=2 rm x", "rm", false, false, []string{"x"}},
		{"nohup nice -n 5 rm x", "rm", false, false, []string{"x"}},
		{"timeout 30 rm x", "rm", false, false, []string{"x"}},
		{"command rm x", "rm", false, false, []string{"x"}},
		{"xargs -n 1 rm -f", "rm", false, true, []string{"-f"}},
		{"sudo env A=1 xargs rm", "rm", true, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			c := find(mustParse(t, tt.line), tt.name)
			if c == nil {
				t.Fatalf("no %q command found", tt.name)
			}
			if c.Sudo != tt.sudo || c.ViaXargs != tt.xargs {
				t.Errorf("sudo=%v xargs=%v, want sudo=%v xargs=%v", c.Sudo, c.ViaXargs, tt.sudo, tt.xargs)
			}
			if !slices.Equal(c.Args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", c.Args, tt.wantArgs)
			}
		})
	}
}

func TestNestedContext(t *testing.T) {
	if c := find(mustParse(t, "ssh prod-1 'rm -rf /srv/data'"), "rm"); c == nil || c.Remote != "prod-1" {
		t.Errorf("ssh inner command should be marked remote prod-1, got %+v", c)
	}
	if c := find(mustParse(t, "ssh -p 2222 -i key host 'ls'"), "ls"); c == nil || c.Remote != "host" {
		t.Errorf("ssh options with values should be skipped, got %+v", c)
	}
	if c := find(mustParse(t, `find / -name '*.tmp' -exec rm {} \;`), "rm"); c == nil || !c.ViaFind {
		t.Errorf("find -exec command should be marked ViaFind, got %+v", c)
	}
	if c := find(mustParse(t, "bash <(curl -s http://x.example/install)"), "bash"); c == nil || !c.Downloads {
		t.Errorf("process substitution that downloads should set Downloads, got %+v", c)
	}
}

func TestRedirects(t *testing.T) {
	c := find(mustParse(t, "echo hi > out.txt 2>> err.log"), "echo")
	want := []Redirect{{">", "out.txt"}, {">>", "err.log"}}
	if c == nil || !reflect.DeepEqual(c.Redirs, want) {
		t.Fatalf("redirects = %+v, want %v", c, want)
	}
	// A redirect with no command still has to be seen: `> file` truncates it.
	r := mustParse(t, "> important.db")
	if len(r.Pipelines) != 1 || r.Pipelines[0].Cmds[0].Name != "" || len(r.Pipelines[0].Cmds[0].Redirs) != 1 {
		t.Errorf("bare redirect not captured: %+v", r.Pipelines)
	}
}

func TestVariablesAndQuotesAreFlattened(t *testing.T) {
	c := find(mustParse(t, `rm -rf "$HOME/my dir" '/tmp/a b' ~/x`), "rm")
	want := []string{"-rf", "$HOME/my dir", "/tmp/a b", "~/x"}
	if c == nil || !reflect.DeepEqual(c.Args, want) {
		t.Errorf("args = %v, want %v", c.Args, want)
	}
}

func TestForkBomb(t *testing.T) {
	for _, line := range []string{":(){ :|:& };:", "bomb(){ bomb|bomb& }; bomb"} {
		if !mustParse(t, line).ForkBomb {
			t.Errorf("%q not detected as a fork bomb", line)
		}
	}
	if mustParse(t, "f(){ echo hi; }; f").ForkBomb {
		t.Error("harmless function flagged as a fork bomb")
	}
}

func TestBackgroundFlag(t *testing.T) {
	if c := find(mustParse(t, "sleep 100 &"), "sleep"); c == nil || !c.Background {
		t.Errorf("background not recorded: %+v", c)
	}
}

func TestParseErrors(t *testing.T) {
	for _, line := range []string{"echo 'unterminated", "if then", "echo $(", "a | | b"} {
		if _, err := Parse(line); err == nil {
			t.Errorf("Parse(%q) should fail", line)
		}
	}
}

func TestNestingDepthIsBounded(t *testing.T) {
	line := "rm x"
	for i := 0; i < 7; i++ { // one more than maxDepth; each level quadruples the length
		line = "sh -c '" + strings.ReplaceAll(line, "'", `'\''`) + "'"
	}
	if _, err := Parse(line); err != nil {
		t.Fatalf("deeply nested input must not error or hang: %v", err)
	}
}

func TestCommandString(t *testing.T) {
	c := Command{Name: "rm", Args: []string{"-rf", "x"}}
	if got := c.String(); got != "rm -rf x" {
		t.Errorf("String() = %q", got)
	}
	c.Sudo = true
	if got := c.String(); got != "sudo rm -rf x" {
		t.Errorf("String() with sudo = %q, want the sudo prefix kept", got)
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{"ls", "rm -rf /", "a | b && c", "bash -c 'x'", ":(){ :|:& };:", "echo $(echo `x`)", "ssh h 'y'", "find . -exec rm {} +"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = Parse(s) // must never panic or hang
	})
}
