package sandbox

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
)

var (
	usernsOnce sync.Once
	usernsOK   bool
)

// usernsAvailable reports whether unprivileged user+net namespaces can be created.
// It probes once by re-exec'ing this binary as "__probe" inside a new userns+netns;
// if the clone succeeds the child reaches main and exits 0 (see cmd/faraday/main.go).
func usernsAvailable() bool {
	usernsOnce.Do(func() {
		self, err := os.Executable()
		if err != nil {
			self = "/proc/self/exe"
		}
		cmd := exec.Command(self, "__probe")
		// No env needed: the "__probe" child exits 0 immediately if the clone worked.
		cmd.Env = []string{}
		cmd.SysProcAttr = cloneAttr()
		usernsOK = cmd.Run() == nil
	})
	return usernsOK
}

// cloneAttr returns the SysProcAttr that creates a new user+net namespace with an
// identity uid/gid mapping (so X11/Wayland/PipeWire credentials keep working).
func cloneAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: os.Getuid(), HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: os.Getgid(), HostID: os.Getgid(), Size: 1},
		},
		GidMappingsEnableSetgroups: false,
	}
}
