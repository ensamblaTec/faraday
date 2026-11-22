// Package sandbox runs a command with its network isolated, picking a backend.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// Spec describes a sandboxed launch.
type Spec struct {
	Cmd          []string // command + args (non-flatpak)
	Loopback     bool     // bring up the namespace-internal lo
	Flatpak      bool     // use the flatpak backend
	FlatpakAppID string   // flatpak application id
	FlatpakArgs  []string // extra args after the app id
	ForceBwrap   bool     // skip the userns backend (testing / fallback)
	Force        bool     // flatpak: launch even if an instance is already running (skip the hand-off guard)
	Env          []string `json:"-"` // child environment (nil → os.Environ()); passed via the process env, not the child JSON payload
}

// Runner executes a Spec with the network blocked.
type Runner interface {
	Run(ctx context.Context, spec Spec) error
	Available() bool
	Name() string
}

// ErrNoBackend is returned when no isolation backend is usable.
var ErrNoBackend = errors.New("faraday: no usable sandbox backend (unprivileged userns disabled and bwrap not found)")

// Select picks the most appropriate backend for spec.
func Select(spec Spec) (Runner, error) {
	if spec.Flatpak {
		r := &flatpakRunner{}
		if r.Available() {
			return r, nil
		}
		return nil, errors.New("faraday: flatpak requested but flatpak not installed")
	}
	if !spec.ForceBwrap {
		if r := (&nsRunner{}); r.Available() {
			return r, nil
		}
	}
	if r := (&bwrapRunner{}); r.Available() {
		return r, nil
	}
	return nil, ErrNoBackend
}

// envOrDefault returns env, or the current process environment when env is nil.
// Shared by every backend, so it lives here rather than in one runner's file.
func envOrDefault(env []string) []string {
	if env == nil {
		return os.Environ()
	}
	return env
}

// warnLoopbackIgnored notes on w that --loopback is a no-op for backend when
// loopback was requested. Only the netns backend brings the namespace-internal
// lo up; bwrap and flatpak manage their own network namespace, so honoring the
// flag there would be misleading. Warning (instead of failing) keeps the run
// working while making the ignored flag visible.
func warnLoopbackIgnored(w io.Writer, backend string, loopback bool) {
	if !loopback {
		return
	}
	fmt.Fprintf(w, "faraday: --loopback has no effect on the %s backend (only the netns backend configures loopback); continuing without it\n", backend)
}
