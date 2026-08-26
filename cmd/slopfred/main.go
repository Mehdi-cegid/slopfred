// Command slopfred is a thin CLI over the slopfred core API. It parses
// arguments and calls the in-process core; all behaviour lives in the core.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

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
		return fmt.Errorf("usage: slopfred <command> [args]\ncommands: init, add")
	}
	switch args[0] {
	case "init":
		return runInit(args[1:], out)
	case "add":
		return runAdd(args[1:], out)
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

// runAdd wires `slopfred add <arg>` to the core: a local folder path routes to
// slopfred.Add, an upstream git URL (optionally with a #subpath) to
// slopfred.AddUpstream.
func runAdd(args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: slopfred add <skill-folder-path>|<git-url>[#subpath]")
	}
	if url, subpath, ok := parseUpstreamArg(args[0]); ok {
		res, err := slopfred.AddUpstream(url, subpath)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "added skill %q (origin: upstream %s)\n", res.Name, url)
		return nil
	}
	res, err := slopfred.Add(args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "added skill %q (origin: local)\n", res.Name)
	return nil
}

// parseUpstreamArg splits an add argument into an upstream git URL and optional
// subpath, reporting whether it should be treated as upstream at all. The
// `<url>#subpath` form is upstream by construction; a bare argument is upstream
// only when it looks like a git URL, so local folder paths still route to Add.
func parseUpstreamArg(arg string) (url, subpath string, ok bool) {
	base := arg
	if i := strings.IndexByte(arg, '#'); i >= 0 {
		base, subpath = arg[:i], arg[i+1:]
	}
	if subpath != "" || looksLikeGitURL(base) {
		return base, subpath, true
	}
	return "", "", false
}

// looksLikeGitURL reports whether s is a git remote rather than a local path:
// a scheme URL (git://, https://, file://, …), an scp-like remote
// (user@host:path), or a path ending in .git.
func looksLikeGitURL(s string) bool {
	if strings.Contains(s, "://") {
		return true
	}
	if at := strings.IndexByte(s, '@'); at > 0 {
		if colon := strings.IndexByte(s, ':'); colon > at {
			return true
		}
	}
	return strings.HasSuffix(s, ".git")
}
