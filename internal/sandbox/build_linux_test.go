//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// TestMain intercepts the same re-exec sentinels cmd/faraday/main.go handles.
//
// usernsAvailable() and nsRunner.Run() find "self" via os.Executable(). In
// production that resolves to the faraday binary, which understands
// "__probe"/"__child". Under `go test`, self resolves to this package's
// compiled test binary instead. Without this interception, exec.Command(self,
// "__probe") would fall through to the test binary's default behavior of
// running every test in the package (no -test.run filter is propagated to
// the re-exec'd child) — which includes the very tests that call
// usernsAvailable() again, recursively. Each level also clones a fresh
// userns/netns, so left unchecked this is a genuine fork bomb: a process
// tree of dozens-to-hundreds of nested "sandbox.test __probe" children was
// observed in practice before this fix.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "__probe":
			os.Exit(0)
		case "__child":
			if err := RunChild(); err != nil {
				fmt.Fprintln(os.Stderr, "faraday:", err)
				os.Exit(1)
			}
			os.Exit(0) // unreachable on success: RunChild execs.
		}
	}
	code := m.Run()
	if buildDir != "" {
		os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

var (
	buildOnce sync.Once
	binPath   string
	buildDir  string
	buildErr  error
)

// buildBinary builds the faraday binary once and reuses it across every test
// in this package that needs it. The binary lives in a directory created
// with os.MkdirTemp (not t.TempDir()): t.TempDir() ties cleanup to the
// *testing.T of whichever test first calls buildBinary, so once that test
// finishes Go removes its temp dir out from under every later test still
// holding the cached binPath. TestMain removes buildDir once after the whole
// package's tests finish instead.
func buildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "faraday-test-bin-")
		if err != nil {
			buildErr = err
			return
		}
		buildDir = dir
		binPath = filepath.Join(dir, "faraday")
		out, err := exec.Command("go", "build", "-o", binPath, "github.com/ensamblatec/faraday/cmd/faraday").CombinedOutput()
		if err != nil {
			buildErr = err
			t.Logf("build output:\n%s", out)
		}
	})
	if buildErr != nil {
		t.Fatalf("build faraday: %v", buildErr)
	}
	return binPath
}
