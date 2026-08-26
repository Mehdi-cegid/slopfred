// Command slopfred is a thin CLI over the slopfred core API. It parses
// arguments and calls the in-process core; all behaviour lives in the core.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mehdi/slopfred/internal/slopfred"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "slopfred:", err)
		os.Exit(1)
	}
}

// run dispatches a single subcommand. It is the seam the CLI tests drive.
func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: slopfred <command> [args]\ncommands: init")
	}
	switch args[0] {
	case "init":
		return runInit(args[1:], out)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// runInit wires `slopfred init <remote>` to slopfred.Init.
func runInit(args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: slopfred init <git-remote-url>")
	}
	res, err := slopfred.Init(args[0])
	if err != nil {
		return err
	}
	if res.Created {
		fmt.Fprintf(out, "initialised slopfred store at %s\n", res.Store.Root)
	} else {
		fmt.Fprintf(out, "slopfred store already present at %s (remote updated)\n", res.Store.Root)
	}
	return nil
}
