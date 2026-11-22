package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type flatpakRunner struct{}

func (r *flatpakRunner) Name() string { return "flatpak" }

func (r *flatpakRunner) Available() bool {
	_, err := exec.LookPath("flatpak")
	return err == nil
}

// flatpakPS returns `flatpak ps` output as one app-id per line. It's a package
// variable so tests can inject canned output without a real flatpak.
var flatpakPS = func() ([]byte, error) {
	return exec.Command("flatpak", "ps", "--columns=application").Output()
}

// parseFlatpakPs reports whether appID appears as a whole line in flatpak ps
// output (one application id per line). Matching whole lines avoids treating
// "com.example.App" as running when only "com.example.AppExtra" is.
func parseFlatpakPs(psOutput []byte, appID string) bool {
	for line := range strings.SplitSeq(string(psOutput), "\n") {
		if strings.TrimSpace(line) == appID {
			return true
		}
	}
	return false
}

// flatpakInstanceRunning reports whether an instance of appID is already
// running. On any error querying flatpak it returns false (fail-open on the
// probe: a missing/erroring `flatpak ps` shouldn't block a legitimate launch).
func flatpakInstanceRunning(appID string) bool {
	out, err := flatpakPS()
	if err != nil {
		return false
	}
	return parseFlatpakPs(out, appID)
}

func (r *flatpakRunner) Run(ctx context.Context, spec Spec) error {
	warnLoopbackIgnored(os.Stderr, r.Name(), spec.Loopback)
	// Single-instance apps (Spotify, Chromium/Electron apps) hand a second
	// launch off to the copy that's already running and exit. If that copy was
	// started with network, `flatpak run --unshare=network` would silently
	// attach to it instead of caging anything — a false sense of isolation.
	// Refuse rather than mislead, unless --force is given.
	if !spec.Force && flatpakInstanceRunning(spec.FlatpakAppID) {
		return fmt.Errorf("faraday: %s is already running; launching again would hand off to that (possibly networked) instance instead of a caged one. Close it first (flatpak kill %s) or pass --force to launch anyway", spec.FlatpakAppID, spec.FlatpakAppID)
	}
	cmd := exec.CommandContext(ctx, "flatpak", flatpakArgs(spec)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = envOrDefault(spec.Env)
	return cmd.Run()
}

// flatpakArgs builds the `flatpak run` argument list for spec. It deliberately
// omits --die-with-parent: flatpak owns the app's network namespace regardless
// of faraday, so tying the app's lifetime to faraday adds no isolation — and it
// breaks single-instance apps (e.g. Spotify) whose `flatpak run` returns as soon
// as it hands off, which would let --die-with-parent reap the app immediately.
func flatpakArgs(spec Spec) []string {
	args := []string{"run", "--unshare=network", spec.FlatpakAppID}
	return append(args, spec.FlatpakArgs...)
}

// DetectFlatpak reports whether appID is an installed flatpak.
func DetectFlatpak(appID string) bool {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return false
	}
	return exec.Command("flatpak", "info", appID).Run() == nil
}
