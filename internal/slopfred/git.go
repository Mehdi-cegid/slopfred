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
