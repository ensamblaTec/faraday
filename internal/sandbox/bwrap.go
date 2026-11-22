package sandbox

import (
	"context"
	"os"
	"os/exec"
)

type bwrapRunner struct{}

func (r *bwrapRunner) Name() string { return "bwrap" }

func (r *bwrapRunner) Available() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

func (r *bwrapRunner) Run(ctx context.Context, spec Spec) error {
	warnLoopbackIgnored(os.Stderr, r.Name(), spec.Loopback)
	args := []string{"--unshare-net", "--die-with-parent", "--dev-bind", "/", "/", "--"}
	args = append(args, spec.Cmd...)
	cmd := exec.CommandContext(ctx, "bwrap", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = envOrDefault(spec.Env)
	return cmd.Run()
}
