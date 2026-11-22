package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ensamblatec/faraday/internal/profile"
	"github.com/ensamblatec/faraday/internal/sandbox"
)

type runPlan struct {
	cmd         []string // ad-hoc command (after --); nil when using a profile
	profileName string   // profile to load; "" when ad-hoc
	loopback    bool     // --loopback flag
	force       bool     // --force flag (flatpak: launch even if an instance is already running)
}

// splitArgs separates args before "--" from the command after it.
func splitArgs(args []string) (before, cmd []string, hasSep bool) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:], true
		}
	}
	return args, nil, false
}

// parseRun parses `run` arguments into a runPlan (no side effects).
func parseRun(args []string) (runPlan, error) {
	before, cmd, hasSep := splitArgs(args)
	var rp runPlan
	var positionals []string
	for _, a := range before {
		switch {
		case a == "--loopback":
			rp.loopback = true
		case a == "--force":
			rp.force = true
		case strings.HasPrefix(a, "-"):
			return rp, fmt.Errorf("unknown flag %q", a)
		default:
			positionals = append(positionals, a)
		}
	}
	switch {
	case hasSep:
		if len(cmd) == 0 {
			return rp, errors.New("no command after --")
		}
		rp.cmd = cmd
		return rp, nil
	case len(positionals) == 1:
		rp.profileName = positionals[0]
		return rp, nil
	default:
		return rp, errors.New("usage: faraday run [--loopback] (<profile> | -- <cmd> [args...])")
	}
}

// SpecFromProfile builds a sandbox.Spec from a saved profile.
func SpecFromProfile(p *profile.Profile) sandbox.Spec {
	spec := sandbox.Spec{Loopback: p.Loopback, Cmd: p.Cmd}
	if p.Flatpak != "" {
		spec.Flatpak = true
		spec.FlatpakAppID = p.Flatpak
	}
	return spec
}

func cmdRun(args []string) int {
	rp, err := parseRun(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "faraday run:", err)
		return 2
	}
	var spec sandbox.Spec
	if rp.profileName != "" {
		p, err := profile.Load(rp.profileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "faraday run: load profile %q: %v\n", rp.profileName, err)
			return 1
		}
		spec = SpecFromProfile(p)
		if rp.loopback {
			spec.Loopback = true
		}
	} else {
		spec = sandbox.Spec{Cmd: rp.cmd, Loopback: rp.loopback}
	}
	spec.Force = rp.force

	runner, err := sandbox.Select(spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "faraday run:", err)
		return 1
	}
	if err := runner.Run(context.Background(), spec); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "faraday run:", err)
		return 1
	}
	return 0
}

func cmdList(args []string) int {
	names, err := profile.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "faraday list:", err)
		return 1
	}
	if len(names) == 0 {
		fmt.Println("no profiles yet (add one with: faraday save <name> ...)")
		return 0
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return 0
}

func cmdSave(args []string) int {
	before, cmd, hasSep := splitArgs(args)
	p := &profile.Profile{}
	var positionals []string
	for i := 0; i < len(before); i++ {
		switch before[i] {
		case "--loopback":
			p.Loopback = true
		case "--flatpak":
			if i+1 >= len(before) {
				fmt.Fprintln(os.Stderr, "faraday save: --flatpak needs an app id")
				return 2
			}
			i++
			p.Flatpak = before[i]
		default:
			positionals = append(positionals, before[i])
		}
	}
	if len(positionals) != 1 {
		fmt.Fprintln(os.Stderr, "usage: faraday save <name> [--loopback] (--flatpak <appid> | -- <cmd>...)")
		return 2
	}
	switch {
	case p.Flatpak != "" && hasSep && len(cmd) > 0:
		// Both a flatpak app id and an ad-hoc command were given; the two are
		// mutually exclusive, so reject rather than silently dropping the command.
		fmt.Fprintln(os.Stderr, "faraday save: give either --flatpak <appid> or -- <cmd>..., not both")
		return 2
	case p.Flatpak != "":
		// ok
	case hasSep && len(cmd) > 0:
		p.Cmd = cmd
	default:
		fmt.Fprintln(os.Stderr, "faraday save: provide --flatpak <appid> or -- <cmd>...")
		return 2
	}
	if err := profile.Save(positionals[0], p); err != nil {
		fmt.Fprintln(os.Stderr, "faraday save:", err)
		return 1
	}
	fmt.Printf("saved profile %q\n", positionals[0])
	return 0
}
