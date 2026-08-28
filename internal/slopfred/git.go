package slopfred

import (
	"fmt"
	"os/exec"
	"strings"
)

// git runs a git command in dir and returns its trimmed stdout, or an error
// that includes git's stderr for diagnosis.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// isGitRepo reports whether dir is the top level of a git working tree.
func isGitRepo(dir string) bool {
	out, err := git(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// remoteURL returns the URL of the named remote, or "" if it is not set.
func remoteURL(dir, name string) string {
	out, err := git(dir, "remote", "get-url", name)
	if err != nil {
		return ""
	}
	return out
}

// setRemote sets remote name to url, replacing any existing value.
func setRemote(dir, name, url string) error {
	if remoteURL(dir, name) == "" {
		_, err := git(dir, "remote", "add", name, url)
		return err
	}
	_, err := git(dir, "remote", "set-url", name, url)
	return err
}

// currentBranch returns the checked-out branch name, or "" when the working
// tree has no commits yet (a fresh git init sits on an unborn branch).
func currentBranch(dir string) string {
	out, err := git(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || out == "HEAD" {
		return ""
	}
	return out
}

// hasCommits reports whether dir has at least one commit on its current branch.
func hasCommits(dir string) bool {
	_, err := git(dir, "rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

// remoteHasBranch reports whether the named branch exists on remote. It is how
// sync tells a first-ever push (empty remote) from a later pull-then-push.
func remoteHasBranch(dir, remote, branch string) bool {
	out, err := git(dir, "ls-remote", "--heads", remote, branch)
	return err == nil && strings.TrimSpace(out) != ""
}
