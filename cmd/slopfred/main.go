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
		return fmt.Errorf("usage: slopfred <command> [args]\ncommands: init, add, pack, activate, sync")
	}
	switch args[0] {
	case "init":
		return runInit(args[1:], out)
	case "add":
		return runAdd(args[1:], out)
	case "pack":
		return runPack(args[1:], out)
	case "activate":
		return runActivate(args[1:], out)
	case "sync":
		return runSync(args[1:], out)
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

// runPack wires `slopfred pack` curation to the core:
//
//	pack create <name>          create a named, empty pack
//	pack add <pack> <skill>     add a skill reference to a pack
//	pack remove <pack> <skill>  remove a skill reference from a pack
//	pack list                   list pack names
func runPack(args []string, out io.Writer) error {
	if len(args) == 0 {
		return packUsage()
	}
	switch args[0] {
	case "create":
		if len(args) != 2 {
			return fmt.Errorf("usage: slopfred pack create <name>")
		}
		res, err := slopfred.CreatePack(args[1])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "created pack %q\n", res.Name)
		return nil
	case "add":
		if len(args) != 3 {
			return fmt.Errorf("usage: slopfred pack add <pack> <skill>")
		}
		res, err := slopfred.AddRef(args[1], args[2])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "pack %q now references: %s\n", res.Name, joinRefs(res.Refs))
		return nil
	case "remove":
		if len(args) != 3 {
			return fmt.Errorf("usage: slopfred pack remove <pack> <skill>")
		}
		res, err := slopfred.RemoveRef(args[1], args[2])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "pack %q now references: %s\n", res.Name, joinRefs(res.Refs))
		return nil
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("usage: slopfred pack list")
		}
		names, err := slopfred.ListPacks()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Fprintln(out, "no packs")
			return nil
		}
		for _, name := range names {
			fmt.Fprintln(out, name)
		}
		return nil
	default:
		return packUsage()
	}
}

// packUsage reports the pack subcommand grammar.
func packUsage() error {
	return fmt.Errorf("usage: slopfred pack <create|add|remove|list> [args]")
}

// runActivate wires `slopfred activate <pack> --scope user|project` to
// slopfred.Activate. Project scope populates the current working directory's
// discovery trees; user scope populates the home dir's.
func runActivate(args []string, out io.Writer) error {
	pack, scope, err := parseActivateArgs(args)
	if err != nil {
		return err
	}
	root := ""
	if scope == "project" {
		if root, err = os.Getwd(); err != nil {
			return fmt.Errorf("resolving working directory: %w", err)
		}
	}
	res, err := slopfred.Activate(pack, scope, root)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "activated pack %q (%s scope): placed %s into %d discovery paths\n",
		res.Pack, res.Scope, joinRefs(res.Folders), len(res.Targets))
	return nil
}

// runSync wires `slopfred sync` to slopfred.Sync: a git pull then push of the
// store against its configured remote. It takes no arguments.
func runSync(args []string, out io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: slopfred sync")
	}
	res, err := slopfred.Sync()
	if err != nil {
		return err
	}
	if res.Committed {
		fmt.Fprintf(out, "synced store (branch %s): snapshotted local changes and pushed\n", res.Branch)
	} else {
		fmt.Fprintf(out, "synced store (branch %s): no local changes, pulled and pushed\n", res.Branch)
	}
	return nil
}

// parseActivateArgs pulls the pack name and required --scope flag out of the
// activate arguments in any order.
func parseActivateArgs(args []string) (pack, scope string, err error) {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--scope":
			if i+1 >= len(args) {
				return "", "", activateUsage()
			}
			scope = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--scope="):
			scope = strings.TrimPrefix(args[i], "--scope=")
		case pack == "":
			pack = args[i]
		default:
			return "", "", activateUsage()
		}
	}
	if pack == "" || scope == "" {
		return "", "", activateUsage()
	}
	return pack, scope, nil
}

// activateUsage reports the activate command grammar.
func activateUsage() error {
	return fmt.Errorf("usage: slopfred activate <pack> --scope user|project")
}

// joinRefs renders a pack's refs for CLI output.
func joinRefs(refs []string) string {
	if len(refs) == 0 {
		return "(empty)"
	}
	return strings.Join(refs, ", ")
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
