// Package cli implements faraday's command-line dispatch.
package cli

import (
	"fmt"
	"io"
	"os"
)

const version = "0.1.0"

// Main dispatches a faraday subcommand and returns the process exit code.
func Main(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "run":
		return cmdRun(args[1:])
	case "list":
		return cmdList(args[1:])
	case "save":
		return cmdSave(args[1:])
	case "version", "--version", "-v":
		fmt.Println("faraday", version)
		return 0
	case "help", "--help", "-h":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "faraday: unknown command %q\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `faraday — run apps inside a Faraday cage (zero network)

usage:
  faraday run [--loopback] [--force] -- <cmd> [args...]   run a command, no network
  faraday run [--loopback] [--force] <profile>            run a saved profile
  faraday list                                            list saved profiles
  faraday save <name> [--loopback] (--flatpak <appid> | -- <cmd>...)
  faraday version | help

flags:
  --loopback   bring up the sandbox-internal lo (default: lo is down)
  --force      flatpak: launch even if an instance is already running
               (by default faraday refuses, to avoid handing off to a
                possibly-networked instance — see README)
`)
}
