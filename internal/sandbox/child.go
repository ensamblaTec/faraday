package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// RunChild runs inside the freshly-created user+net namespace. It optionally
// brings up loopback, then execs the target, replacing this process.
func RunChild() error {
	payload := os.Getenv(childEnvVar)
	if payload == "" {
		return fmt.Errorf("faraday: %s not set", childEnvVar)
	}
	var spec Spec
	if err := json.Unmarshal([]byte(payload), &spec); err != nil {
		return fmt.Errorf("faraday: decode child spec: %w", err)
	}
	if len(spec.Cmd) == 0 {
		return fmt.Errorf("faraday: empty command in child")
	}
	if spec.Loopback {
		if err := bringLoopbackUp(); err != nil {
			return fmt.Errorf("faraday: bring up lo: %w", err)
		}
		// Drop every capability set (effective/permitted/inheritable/ambient)
		// granted for bringLoopbackUp before exec'ing the sandboxed target:
		// ambient capabilities (and the inheritable bit needed to raise them)
		// survive execve, and the user's program has no business holding
		// network capabilities inside its otherwise-isolated netns.
		if err := dropAllCaps(); err != nil {
			return fmt.Errorf("faraday: drop capabilities: %w", err)
		}
	}
	path, err := exec.LookPath(spec.Cmd[0])
	if err != nil {
		return fmt.Errorf("faraday: command not found: %s", spec.Cmd[0])
	}
	return unix.Exec(path, spec.Cmd, childEnviron())
}

// bringLoopbackUp sets the namespace-internal lo interface up.
func bringLoopbackUp() error {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return err
	}
	return netlink.LinkSetUp(lo)
}

// dropAllCaps clears the effective, permitted, and inheritable capability
// sets on the current process; the kernel clears any ambient bits that are
// no longer a subset of permitted/inheritable as a side effect.
func dropAllCaps() error {
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	return unix.Capset(&hdr, &data[0])
}

// childEnviron returns the current environment without the FARADAY_CHILD sentinel.
func childEnviron() []string {
	all := os.Environ()
	out := make([]string, 0, len(all))
	for _, kv := range all {
		if strings.HasPrefix(kv, childEnvVar+"=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
