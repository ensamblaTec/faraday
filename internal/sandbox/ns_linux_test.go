//go:build linux

package sandbox

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
)

// connFailureMarkers are curl error strings that indicate the request never
// got past connection setup (as opposed to, say, a DNS hiccup unrelated to
// network isolation). Different curl/libcurl builds phrase this differently.
var connFailureMarkers = []string{
	"Couldn't connect",
	"Could not resolve",
	"Failed to connect",
}

// runViaFaraday builds the binary once and runs `faraday run -- <args>` capturing stdout.
func runViaFaraday(t *testing.T, args ...string) (string, error) {
	t.Helper()
	bin := buildBinary(t)
	full := append([]string{"run", "--"}, args...)
	var out bytes.Buffer
	cmd := exec.CommandContext(context.Background(), bin, full...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func TestNetnsOnlyLoopback(t *testing.T) {
	if !usernsAvailable() {
		t.Skip("unprivileged userns not available")
	}
	out, err := runViaFaraday(t, "ip", "-o", "link", "show")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	// Only the loopback interface should be present.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 || !strings.Contains(out, " lo:") {
		t.Fatalf("expected only lo, got:\n%s", out)
	}
}

func TestNetnsBlocksNetwork(t *testing.T) {
	if !usernsAvailable() {
		t.Skip("unprivileged userns not available")
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not found in PATH")
	}
	out, err := runViaFaraday(t, "curl", "-sS", "--max-time", "5", "https://example.com")
	if err == nil {
		t.Fatalf("curl unexpectedly succeeded inside sandbox:\n%s", out)
	}
	// The command must fail specifically because the network is gone, not
	// for some unrelated reason (e.g. curl itself missing or misbehaving):
	// check for a connection-level error message and/or curl's documented
	// exit codes for "couldn't connect" (7) and "couldn't resolve host" (6).
	exitCode := -1
	if ee, ok := err.(*exec.ExitError); ok {
		exitCode = ee.ExitCode()
	}
	hasMarker := false
	for _, m := range connFailureMarkers {
		if strings.Contains(out, m) {
			hasMarker = true
			break
		}
	}
	if !hasMarker && exitCode != 6 && exitCode != 7 {
		t.Fatalf("curl failed, but not with a recognizable connection-level error (exit=%d):\n%s", exitCode, out)
	}
}
