// Command faraday runs applications with their network fully blocked.
package main

import (
	"fmt"
	"os"

	"github.com/ensamblatec/faraday/internal/cli"
	"github.com/ensamblatec/faraday/internal/sandbox"
)

func main() {
	// Re-exec sentinels must be handled before normal dispatch.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "__probe":
			// Reached only if the new userns+netns was created successfully.
			os.Exit(0)
		case "__child":
			if err := sandbox.RunChild(); err != nil {
				fmt.Fprintln(os.Stderr, "faraday:", err)
				os.Exit(1)
			}
			return // unreachable: RunChild execs on success.
		}
	}
	os.Exit(cli.Main(os.Args[1:]))
}
