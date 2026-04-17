package project

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DetectRemote runs `git -C <cwd> remote get-url origin` and returns the
// trimmed remote URL. Returns an error if git is missing, cwd is not a
// git repo, origin is not configured, or the command produces empty output.
//
// This is intentionally a shell-out rather than a library dependency:
// docsiq targets environments that already have git installed (the whole
// product is aimed at AI coding agents working in git repos), and shelling
// out avoids pulling in go-git (~4 MB of transitive deps) just to read one
// config value.
func DetectRemote(cwd string) (string, error) {
	if strings.TrimSpace(cwd) == "" {
		return "", fmt.Errorf("detect remote: cwd is empty")
	}

	cmd := exec.Command("git", "-C", cwd, "remote", "get-url", "origin")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("detect remote: git remote get-url origin failed: %s", msg)
	}

	remote := strings.TrimSpace(stdout.String())
	if remote == "" {
		return "", fmt.Errorf("detect remote: origin is unset in %s", cwd)
	}
	return remote, nil
}
