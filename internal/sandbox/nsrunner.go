package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"
)

// childEnvVar carries the JSON-encoded Spec to the re-exec'd child.
const childEnvVar = "FARADAY_CHILD"

type nsRunner struct{}

func (r *nsRunner) Name() string    { return "netns" }
func (r *nsRunner) Available() bool { return usernsAvailable() }

func (r *nsRunner) Run(ctx context.Context, spec Spec) error {
	if len(spec.Cmd) == 0 {
		return fmt.Errorf("faraday: empty command")
	}
	payload, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("faraday: marshal spec: %w", err)
	}
	self, err := os.Executable()
	if err != nil {
		self = "/proc/self/exe"
	}
	cmd := exec.CommandContext(ctx, self, "__child")
	cmd.Env = append(envOrDefault(spec.Env), childEnvVar+"="+string(payload))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.SysProcAttr = cloneAttr()
	// Kill the sandboxed child if this faraday process dies, so a SIGKILL to the
	// parent can't leave an orphaned program still running (the bwrap/flatpak
	// backends get the same guarantee from --die-with-parent).
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	if spec.Loopback {
		// cloneAttr() maps an identity (non-zero) uid/gid into the new userns, so
		// the legacy "euid 0 at exec gets full caps" rule does not apply to the
		// re-exec into "__child" below: the CAP_NET_ADMIN the kernel grants the
		// namespace creator at clone(2) time would otherwise be dropped at that
		// execve. Ambient capabilities are the standard mechanism to carry a
		// specific capability across an execve of a non-setuid binary, so request
		// it here only when the child actually needs to bring lo up; child.go
		// clears the ambient set again before exec'ing the sandboxed target so
		// the user's program never inherits it.
		cmd.SysProcAttr.AmbientCaps = []uintptr{unix.CAP_NET_ADMIN}
	}
	// Pdeathsig is delivered when the OS thread that clone(2)'d the child exits.
	// Pin this goroutine to its thread for the whole clone + wait so the Go
	// runtime can't retire that thread early and kill the child prematurely.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return cmd.Run()
}
