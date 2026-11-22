package cli

import "testing"

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
