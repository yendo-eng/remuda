package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

// ContainerWorkspacePath returns a unique, stable in-container mount path for a
// given host workspace directory.
//
// We avoid mounting every workspace at a fixed path like /workspace because
// some tools (eg. Codex) persist the working directory in session history.
func ContainerWorkspacePath(workspaceAbs string) string {
	base := strings.TrimSpace(filepath.Base(workspaceAbs))
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "workspace"
	}

	// Make the path segment readable but safe: only keep letters/digits and
	// normalize all other characters into dashes.
	var b strings.Builder
	b.Grow(len(base))
	lastDash := false
	for _, r := range base {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == '.':
			if b.Len() == 0 {
				continue
			}
			b.WriteRune(r)
			lastDash = false
		default:
			if b.Len() == 0 || lastDash {
				continue
			}
			b.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-_.")
	if slug == "" {
		slug = "workspace"
	}

	sum := sha256.Sum256([]byte(workspaceAbs))
	hash8 := hex.EncodeToString(sum[:])[:8]

	return path.Join("/workspaces", slug+"-"+hash8)
}

// BuildRunCommand composes the docker run command string for containerized sessions.
// It is exported to allow unit testing without invoking Docker.
func BuildRunCommand(
	workspaceAbs, image string,
	opts []string,
	agentCmd string,
	allocateTTY bool,
	containerName string,
) string {
	containerWS := ContainerWorkspacePath(workspaceAbs)

	var b strings.Builder
	b.WriteString("docker run --rm -it ")
	if strings.TrimSpace(containerName) != "" {
		b.WriteString("--name ")
		b.WriteString(containerName)
		b.WriteString(" ")
	}
	if allocateTTY {
		b.WriteString("-t ")
	}
	// Quote the -v argument to handle spaces in paths.
	b.WriteString("-v \"")
	b.WriteString(workspaceAbs)
	b.WriteString(":")
	b.WriteString(containerWS)
	b.WriteString("\" ")
	b.WriteString("-w ")
	b.WriteString(containerWS)
	b.WriteString(" ")
	// Forward common env vars required by agents and GitHub auth
	b.WriteString("-e OPENAI_API_KEY ")
	// Forward Remuda agent metadata for downstream tooling.
	b.WriteString("-e REMUDA_AGENT ")
	b.WriteString("-e REMUDA_MODEL ")
	b.WriteString("-e GH_TOKEN ")
	b.WriteString("-e GITHUB_TOKEN ")
	b.WriteString("-e GIT_HTTPS_USERNAME ")
	// Prevent interactive git prompts and auto-accept new host keys to avoid hangs
	b.WriteString("-e GIT_TERMINAL_PROMPT=0 ")
	b.WriteString("-e GIT_SSH_COMMAND='ssh -o StrictHostKeyChecking=accept-new' ")
	for _, o := range opts {
		if strings.TrimSpace(o) == "" {
			continue
		}
		b.WriteString(o)
		b.WriteString(" ")
	}
	b.WriteString(image)
	b.WriteString(" ")
	b.WriteString("bash -lc ")
	var inner strings.Builder
	// Configure Git credentials non-interactively. Prefer explicit token if provided.
	inner.WriteString("if [ -n \"$GITHUB_TOKEN\" ] || [ -n \"$GH_TOKEN\" ]; then ")
	inner.WriteString("TOK=\"${GITHUB_TOKEN:-$GH_TOKEN}\"; ")
	inner.WriteString(
		"git config --global credential.helper 'store --file=/root/.git-credentials' >/dev/null 2>&1 || true; ",
	)
	inner.WriteString(
		"USER=\\\"x-access-token\\\"; if command -v gh >/dev/null; then U=$(gh api user -q .login 2>/dev/null || true); [ -n \"$U\" ] && USER=$U; fi; ",
	)
	inner.WriteString(
		"umask 077; printf 'https://%s:%s@github.com\\n' \"$USER\" \"$TOK\" > /root/.git-credentials; ",
	)
	inner.WriteString("else ")
	inner.WriteString(
		"git config --global credential.helper '!gh auth git-credential' >/dev/null 2>&1 || true; ",
	)
	inner.WriteString(
		"if command -v gh >/dev/null; then gh auth setup-git -h github.com >/dev/null 2>&1 || true; fi; ",
	)
	inner.WriteString("fi; ")
	// Prefer SSH-based GitHub URLs to avoid credential leaks and leverage
	// forwarded SSH agents inside the container.
	inner.WriteString(`git config --global url."git@github.com:".insteadOf "https://github.com/" >/dev/null 2>&1 || true; `)
	inner.WriteString(agentCmd)
	b.WriteString(shellSingleQuote(inner.String()))
	return b.String()
}

// BuildPreflightRunCommand composes a non-interactive docker run command that
// exercises the same workspace bind mount and caller-supplied Docker options as
// a real containerized session, then exits immediately. It is used before
// detached launches so mount/file-sharing failures surface before Remuda returns
// success to the caller.
func BuildPreflightRunCommand(workspaceAbs, image string, opts []string) string {
	containerWS := ContainerWorkspacePath(workspaceAbs)

	var b strings.Builder
	b.WriteString("docker run --rm ")
	b.WriteString("-v \"")
	b.WriteString(workspaceAbs)
	b.WriteString(":")
	b.WriteString(containerWS)
	b.WriteString("\" ")
	b.WriteString("-w ")
	b.WriteString(containerWS)
	b.WriteString(" ")
	b.WriteString("-e OPENAI_API_KEY ")
	b.WriteString("-e REMUDA_AGENT ")
	b.WriteString("-e REMUDA_MODEL ")
	b.WriteString("-e GH_TOKEN ")
	b.WriteString("-e GITHUB_TOKEN ")
	b.WriteString("-e GIT_TERMINAL_PROMPT=0 ")
	for _, o := range opts {
		if strings.TrimSpace(o) == "" {
			continue
		}
		b.WriteString(o)
		b.WriteString(" ")
	}
	b.WriteString(image)
	b.WriteString(" ")
	b.WriteString("bash -lc ")
	b.WriteString(shellSingleQuote("true"))
	return b.String()
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
