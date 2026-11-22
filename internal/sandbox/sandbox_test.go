package sandbox

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWarnLoopbackIgnored(t *testing.T) {
	var buf bytes.Buffer
	warnLoopbackIgnored(&buf, "bwrap", true)
	got := buf.String()
	if !strings.Contains(got, "loopback") || !strings.Contains(got, "bwrap") {
		t.Fatalf("warning must mention --loopback and the backend name, got %q", got)
	}

	buf.Reset()
	warnLoopbackIgnored(&buf, "flatpak", false)
	if buf.Len() != 0 {
		t.Fatalf("no warning expected when loopback is not requested, got %q", buf.String())
	}
}

func TestSelectFlatpak(t *testing.T) {
	if _, err := exec.LookPath("flatpak"); err != nil {
		t.Skip("flatpak not installed")
	}
	r, err := Select(Spec{Flatpak: true, FlatpakAppID: "com.example.X"})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if r.Name() != "flatpak" {
		t.Fatalf("got backend %q, want flatpak", r.Name())
	}
}

func TestSelectForceBwrap(t *testing.T) {
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap not installed")
	}
	r, err := Select(Spec{Cmd: []string{"true"}, ForceBwrap: true})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if r.Name() != "bwrap" {
		t.Fatalf("got backend %q, want bwrap", r.Name())
	}
}

func TestDetectFlatpakBogus(t *testing.T) {
	if DetectFlatpak("com.totally.Bogus.NotInstalled") {
		t.Fatal("bogus app id should not be detected as flatpak")
	}
}

func TestParseFlatpakPs(t *testing.T) {
	out := []byte("org.gnome.Calculator\ncom.spotify.Client\nio.example.App\n")
	if !parseFlatpakPs(out, "com.spotify.Client") {
		t.Error("com.spotify.Client is in the ps output; want detected as running")
	}
	if parseFlatpakPs(out, "com.spotify.ClientX") {
		t.Error("must match the whole app id, not a prefix of another line")
	}
	if parseFlatpakPs([]byte(""), "com.spotify.Client") {
		t.Error("empty ps output means nothing is running")
	}
}

func TestFlatpakRunRefusesWhenAlreadyRunning(t *testing.T) {
	orig := flatpakPS
	defer func() { flatpakPS = orig }()
	flatpakPS = func() ([]byte, error) { return []byte("com.spotify.Client\n"), nil }

	err := (&flatpakRunner{}).Run(context.Background(),
		Spec{Flatpak: true, FlatpakAppID: "com.spotify.Client"})
	if err == nil {
		t.Fatal("Run must refuse to launch when an instance is already running (single-instance hand-off risk)")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Fatalf("error should explain the instance is already running, got: %v", err)
	}
}

func TestFlatpakArgsOmitsDieWithParent(t *testing.T) {
	args := flatpakArgs(Spec{FlatpakAppID: "com.spotify.Client"})
	joined := strings.Join(args, " ")
	// --die-with-parent kills single-instance apps (Spotify): flatpak run returns
	// immediately after hand-off, the parent exits, and the app is reaped.
	if strings.Contains(joined, "--die-with-parent") {
		t.Errorf("flatpak args must not include --die-with-parent: %v", args)
	}
	// The network block must remain.
	if !strings.Contains(joined, "--unshare=network") {
		t.Errorf("flatpak args must still block network with --unshare=network: %v", args)
	}
	// flatpak options must precede the app id.
	optIdx, appIdx := -1, -1
	for i, a := range args {
		switch a {
		case "--unshare=network":
			optIdx = i
		case "com.spotify.Client":
			appIdx = i
		}
	}
	if optIdx < 0 || appIdx < 0 || optIdx > appIdx {
		t.Errorf("--unshare=network must come before the app id: %v", args)
	}
}

func TestFlatpakRunForceSkipsRunningCheck(t *testing.T) {
	orig := flatpakPS
	defer func() { flatpakPS = orig }()
	called := false
	flatpakPS = func() ([]byte, error) { called = true; return []byte("com.spotify.Client\n"), nil }

	// A cancelled context makes the eventual launch fail fast; we only care that
	// --force short-circuits the running-instance check (flatpakPS never consulted).
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = (&flatpakRunner{}).Run(ctx,
		Spec{Flatpak: true, FlatpakAppID: "com.spotify.Client", Force: true})
	if called {
		t.Fatal("Force must skip the already-running check")
	}
}

func TestSelectBackendsWithPathIsolation(t *testing.T) {
	t.Run("flatpak unavailable returns error", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := Select(Spec{Flatpak: true, FlatpakAppID: "com.example.App"})
		if err == nil || !strings.Contains(err.Error(), "flatpak") {
			t.Fatalf("expected flatpak unavailable error, got %v", err)
		}
	})

	t.Run("force bwrap unavailable returns ErrNoBackend", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := Select(Spec{Cmd: []string{"true"}, ForceBwrap: true})
		if !errors.Is(err, ErrNoBackend) {
			t.Fatalf("got %v, want ErrNoBackend", err)
		}
	})

	t.Run("force bwrap selects mock bwrap", func(t *testing.T) {
		binDir := t.TempDir()
		bwrapPath := filepath.Join(binDir, "bwrap")
		if err := os.WriteFile(bwrapPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", binDir)
		r, err := Select(Spec{Cmd: []string{"true"}, ForceBwrap: true})
		if err != nil {
			t.Fatalf("Select(ForceBwrap) error = %v", err)
		}
		if r.Name() != "bwrap" {
			t.Fatalf("got backend %q, want bwrap", r.Name())
		}
	})

	t.Run("flatpak selects mock flatpak", func(t *testing.T) {
		binDir := t.TempDir()
		fpPath := filepath.Join(binDir, "flatpak")
		if err := os.WriteFile(fpPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", binDir)
		r, err := Select(Spec{Flatpak: true, FlatpakAppID: "com.example.App"})
		if err != nil {
			t.Fatalf("Select(Flatpak) error = %v", err)
		}
		if r.Name() != "flatpak" {
			t.Fatalf("got backend %q, want flatpak", r.Name())
		}
	})
}
