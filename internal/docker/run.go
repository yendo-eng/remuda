package docker

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/yendo-eng/remuda/internal/util/shell"
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
//
// opts is an argv-style slice of docker run arguments (eg. []string{"-v",
// "/host:/container:ro", "-e", "FOO"}), not pre-joined or pre-quoted shell
// text. The full docker invocation is quoted exactly once here, at the
// session-start boundary, via the canonical shell helper.
func BuildRunCommand(
	workspaceAbs, image string,
	opts []string,
	agentCmd string,
	allocateTTY bool,
	containerName string,
) string {
	containerWS := ContainerWorkspacePath(workspaceAbs)

	argv := []string{"docker", "run", "--rm", "-it"}
	if strings.TrimSpace(containerName) != "" {
		argv = append(argv, "--name", containerName)
	}
	if allocateTTY {
		argv = append(argv, "-t")
	}
	argv = append(argv, "-v", workspaceAbs+":"+containerWS)
	argv = append(argv, "-w", containerWS)
	// Forward common env vars required by agents and GitHub auth
	argv = append(argv, "-e", "OPENAI_API_KEY")
	// Forward Remuda agent metadata for downstream tooling.
	argv = append(argv, "-e", "REMUDA_AGENT")
	argv = append(argv, "-e", "REMUDA_MODEL")
	argv = append(argv, "-e", "GH_TOKEN")
	argv = append(argv, "-e", "GITHUB_TOKEN")
	argv = append(argv, "-e", "GIT_HTTPS_USERNAME")
	// Prevent interactive git prompts and auto-accept new host keys to avoid hangs
	argv = append(argv, "-e", "GIT_TERMINAL_PROMPT=0")
	argv = append(argv, "-e", "GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=accept-new")
	argv = append(argv, opts...)
	argv = append(argv, image)

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
	argv = append(argv, "bash", "-lc", inner.String())

	quoted := make([]string, len(argv))
	for i, tok := range argv {
		quoted[i] = shell.SingleQuote(tok)
	}
	return strings.Join(quoted, " ")
}
