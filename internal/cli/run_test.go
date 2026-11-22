package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ensamblatec/faraday/internal/profile"
)

func TestParseRunAdHoc(t *testing.T) {
	p, err := parseRun([]string{"--loopback", "--", "curl", "https://x"})
	if err != nil {
		t.Fatal(err)
	}
	if p.profileName != "" {
		t.Fatalf("want ad-hoc, got profile %q", p.profileName)
	}
	if !p.loopback || len(p.cmd) != 2 || p.cmd[0] != "curl" {
		t.Fatalf("bad parse: %+v", p)
	}
}

func TestParseRunProfile(t *testing.T) {
	p, err := parseRun([]string{"spotify"})
	if err != nil {
		t.Fatal(err)
	}
	if p.profileName != "spotify" || p.cmd != nil {
		t.Fatalf("want profile spotify, got %+v", p)
	}
}

func TestParseRunNoArgs(t *testing.T) {
	if _, err := parseRun(nil); err == nil {
		t.Fatal("want error for empty run args")
	}
}

func TestParseRunEmptyAfterSep(t *testing.T) {
	if _, err := parseRun([]string{"--"}); err == nil {
		t.Fatal("want error for empty command after --")
	}
}

func TestParseRunForce(t *testing.T) {
	p, err := parseRun([]string{"--force", "spotify"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.force {
		t.Fatalf("want force=true, got %+v", p)
	}
	if p.profileName != "spotify" {
		t.Fatalf("want profile spotify, got %+v", p)
	}
}

func TestCmdSaveRejectsFlatpakAndCmd(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// --flatpak and an ad-hoc -- cmd are mutually exclusive; cmdSave must reject
	// the combination (exit 2) instead of silently dropping the command.
	if got := cmdSave([]string{"both", "--flatpak", "com.example.X", "--", "true"}); got != 2 {
		t.Fatalf("cmdSave(--flatpak + -- cmd) = %d, want 2", got)
	}
}

func TestCmdSaveRejectsUnknownFlag(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)

	if got := cmdSave([]string{"--bogus", "--", "/bin/true"}); got != 2 {
		t.Fatalf("cmdSave(--bogus) = %d, want 2", got)
	}
	bogusPath := filepath.Join(cfgHome, "faraday", "profiles", "--bogus.yaml")
	if _, err := os.Stat(bogusPath); !os.IsNotExist(err) {
		t.Fatalf("unknown flag must not create profile file %s", bogusPath)
	}
}

func TestSpecFromProfile(t *testing.T) {
	cmdProf := &profile.Profile{
		Loopback: true,
		Cmd:      []string{"/usr/bin/env", "sh"},
	}
	cmdSpec := SpecFromProfile(cmdProf)
	if !cmdSpec.Loopback || cmdSpec.Flatpak || cmdSpec.FlatpakAppID != "" || !reflect.DeepEqual(cmdSpec.Cmd, cmdProf.Cmd) {
		t.Fatalf("unexpected Cmd SpecFromProfile: %+v", cmdSpec)
	}

	fpProf := &profile.Profile{
		Flatpak:  "com.example.App",
		Loopback: false,
	}
	fpSpec := SpecFromProfile(fpProf)
	if !fpSpec.Flatpak || fpSpec.FlatpakAppID != "com.example.App" || fpSpec.Loopback {
		t.Fatalf("unexpected Flatpak SpecFromProfile: %+v", fpSpec)
	}
}

func captureStdout(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	code := fn()
	_ = w.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return code, buf.String()
}

func TestCmdList(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	code, out := captureStdout(t, func() int { return cmdList(nil) })
	if code != 0 || !strings.Contains(out, "no profiles yet") {
		t.Fatalf("empty cmdList = (%d, %q), want (0, contains 'no profiles yet')", code, out)
	}

	if err := profile.Save("alpha", &profile.Profile{Cmd: []string{"true"}}); err != nil {
		t.Fatal(err)
	}
	if err := profile.Save("beta", &profile.Profile{Flatpak: "com.example.Beta"}); err != nil {
		t.Fatal(err)
	}

	code, out = captureStdout(t, func() int { return cmdList(nil) })
	if code != 0 || !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("populated cmdList = (%d, %q), want (0, contains alpha & beta)", code, out)
	}
}
