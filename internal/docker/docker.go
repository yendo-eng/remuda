package docker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/util"
)

type Docker interface {
	// Returns an error if docker is not running.
	CheckRunning() error

	// ContainerRunning returns true if the container exists and is running.
	// When the container does not exist, it should return ErrContainerNotFound.
	ContainerRunning(container string) (bool, error)

	// Runs a command inside a running container.
	Exec(container string, command string) error
}

// LoggerSetter allows wiring a per-invocation logger into Docker implementations.
type LoggerSetter interface {
	SetLogger(logger zerolog.Logger)
}

var ErrNotRunning = errors.New("docker is not running")

var ErrContainerNotFound = errors.New("docker container not found")

// BuildContainerAuthOpts returns recommended docker run options to forward
// GitHub and SSH auth into the container, when available on the host.
//
// - Mounts ~/.config/gh to /root/.config/gh (ro) if present.
// - Mounts ~/.gitconfig to /root/.gitconfig (ro) if present.
// - Forwards the SSH agent socket and ~/.ssh (ro) if present.
func BuildContainerAuthOpts() []string {
	return BuildContainerAuthOptsWithProvider(env.Default())
}

// BuildContainerAuthOptsWithProvider returns recommended docker run options to forward
// GitHub and SSH auth into the container, when available on the host.
func BuildContainerAuthOptsWithProvider(provider env.Provider) []string {
	provider = env.OrDefault(provider)
	var opts []string

	home, err := provider.UserHomeDir()
	if err == nil && home != "" {
		ghDir := filepath.Join(home, ".config", "gh")
		if st, err := os.Stat(ghDir); err == nil && st.IsDir() {
			opts = append(opts, "-v "+quote(ghDir)+":"+quote("/root/.config/gh")+":ro")
		}
		// Do not mount host ~/.gitconfig by default. We want the container to
		// be able to write its own /root/.gitconfig so we can configure the
		// gh credential helper non-interactively without touching the host.
		sshDir := filepath.Join(home, ".ssh")
		if st, err := os.Stat(sshDir); err == nil && st.IsDir() {
			opts = append(opts, "-v "+quote(sshDir)+":"+quote("/root/.ssh")+":ro")
		}
	}

	// SSH agent: prefer Docker Desktop's magic path on macOS to avoid
	// mounting launchd sockets which often fails with "operation not supported".
	if runtime.GOOS == "darwin" {
		opts = append(opts, "-v "+quote("/run/host-services/ssh-auth.sock")+":"+quote("/ssh-agent"))
		opts = append(opts, "-e SSH_AUTH_SOCK=/ssh-agent")
	} else if sock := provider.Getenv("SSH_AUTH_SOCK"); strings.TrimSpace(sock) != "" {
		if st, err := os.Stat(sock); err == nil && !st.IsDir() {
			opts = append(opts, "-v "+quote(sock)+":"+quote("/ssh-agent"))
			opts = append(opts, "-e SSH_AUTH_SOCK=/ssh-agent")
		}
	}

	return opts
}

// quote wraps a path in double-quotes to be safe in docker run strings.
func quote(p string) string { return "\"" + p + "\"" }

// BuildGoCacheMountOpts returns docker volume mounts that bind the host's Go build and module caches
// into the container so successive containerized sessions can reuse compiled artifacts.
func BuildGoCacheMountOpts() []string {
	return BuildGoCacheMountOptsWithLogger(logging.DefaultLogger())
}

func BuildGoCacheMountOptsWithLogger(logger zerolog.Logger) []string {
	var opts []string

	for _, mapping := range []struct {
		key    string
		target string
	}{
		{"GOCACHE", "/root/.cache/go-build"},
		{"GOMODCACHE", "/go/pkg/mod"},
	} {
		dir := goEnvPath(logger, mapping.key)
		if dir == "" || strings.EqualFold(dir, "off") {
			continue
		}
		abs := absPath(dir)
		if abs == "" {
			continue
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			logger.Warn().Err(err).Str("path", abs).Str("key", mapping.key).Msg("failed ensuring Go cache directory exists")
			continue
		}
		opts = append(opts, "-v "+quote(abs)+":"+quote(mapping.target))
	}

	return opts
}

func absPath(p string) string {
	if strings.TrimSpace(p) == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return p
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func goEnvPath(logger zerolog.Logger, key string) string {
	out, err := util.RunCmdOutputWithLogger(logger, "go", "env", key)
	if err != nil {
		logger.Warn().Err(err).Str("key", key).Msg("go env lookup failed")
		return ""
	}
	return strings.TrimSpace(out)
}

// ExtraGitMountForWorktree returns a docker `-v` argument that bind-mounts the
// parent repository cache directory (which contains the actual .git directory
// for a worktree) to the same absolute path inside the container. This keeps
// the .git indirection inside the worktree valid when running with --container.
//
// If the expected cache directory does not exist, it returns ("", false).
func ExtraGitMountForWorktree(workspaceAbs string) (string, bool) {
	// Given workspace: <baseDir>/<name>
	// Cache repo lives at: <baseDir>/.repo_cache
	baseDir := filepath.Dir(workspaceAbs)
	cacheDir := filepath.Join(baseDir, ".repo_cache")
	if st, err := os.Stat(cacheDir); err == nil && st.IsDir() {
		// Mount host path to identical path inside container to satisfy the
		// absolute path recorded in the worktree's .git file.
		// Quote the argument to preserve spaces.
		return fmt.Sprintf("-v %q:%q", cacheDir, cacheDir), true
	}
	return "", false
}

// ContainerNameFromSession converts a tmux session name into a Docker-safe container name.
// Docker names must match the regex `[a-zA-Z0-9][a-zA-Z0-9_.-]+`, so we map invalid
// characters to dashes, collapse stray separators, and ensure the first character is
// alphanumeric. Session names like "org/repo/worktree" become "org-repo-worktree".
//
// Importantly, tmux converts dots to underscores in session names (e.g., "5.4" becomes "5_4").
// We replicate this behavior so that the container name derived at launch time matches the
// name derived later from the tmux-reported session name.
func ContainerNameFromSession(session string) string {
	trimmed := strings.TrimSpace(session)
	if trimmed == "" {
		return ""
	}

	// Replace common separators with dashes to preserve the high-level structure.
	// Also convert dots to underscores to match tmux's behavior.
	replaced := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ".", "_").Replace(trimmed)

	var b strings.Builder
	b.Grow(len(replaced))
	lastDash := false
	for _, r := range replaced {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_':
			// Avoid leading separators by skipping until we emit an alphanumeric.
			if b.Len() == 0 {
				continue
			}
			if r == '-' {
				if lastDash {
					continue
				}
				lastDash = true
			} else {
				lastDash = false
			}
			b.WriteRune(r)
		default:
			if b.Len() == 0 || lastDash {
				continue
			}
			b.WriteRune('-')
			lastDash = true
		}
	}

	name := strings.Trim(b.String(), "-_")
	if name == "" {
		return ""
	}

	first := rune(name[0])
	if !unicode.IsLetter(first) && !unicode.IsDigit(first) {
		name = "s-" + name
	}

	// Docker allows up to 128 characters, keep a safety margin to avoid extremely long names.
	const maxLen = 64
	if len(name) > maxLen {
		name = name[:maxLen]
	}
	return name
}

func BuildOpenCodeStateMountOptsWithLogger(logger zerolog.Logger, provider env.Provider) []string {
	provider = env.OrDefault(provider)
	home, err := provider.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}

	return buildOpenCodeStateMountOpts(logger, util.CurrentGOOS(), home)
}

func buildOpenCodeStateMountOpts(logger zerolog.Logger, goos, home string) []string {
	candidates := opencodeStateDirCandidates(goos, home)
	for _, dir := range candidates {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return []string{"-v " + quote(dir) + ":" + quote("/root/.local/share/opencode") + ":rw"}
		}
	}

	// No existing state dir found; create the OS-preferred one so future sessions
	// persist state.
	if len(candidates) == 0 {
		return nil
	}
	preferred := candidates[0]
	if err := os.MkdirAll(preferred, 0o700); err != nil {
		logger.Warn().Err(err).Str("path", preferred).Msg("failed ensuring OpenCode state directory exists; continuing without mount")
		return nil
	}
	return []string{"-v " + quote(preferred) + ":" + quote("/root/.local/share/opencode") + ":rw"}
}

func opencodeStateDirCandidates(goos, home string) []string {
	macDir := filepath.Join(home, "Library", "Application Support", "opencode")
	linuxDir := filepath.Join(home, ".local", "share", "opencode")

	if strings.EqualFold(goos, "darwin") {
		return []string{macDir, linuxDir}
	}
	return []string{linuxDir, macDir}
}

// BuildClaudeStateMountOpts returns docker mount options that expose host Claude
// OAuth/session state in the container, when available.
//
// - Mounts ~/.claude to /root/.claude (rw) when present.
// - Mounts ~/.claude.json to /root/.claude.json (rw) when present.
func BuildClaudeStateMountOpts() []string {
	return BuildClaudeStateMountOptsWithProvider(env.Default())
}

// BuildClaudeStateMountOptsWithProvider returns docker mount options that expose host Claude
// OAuth/session state in the container, when available.
func BuildClaudeStateMountOptsWithProvider(provider env.Provider) []string {
	return BuildClaudeStateMountOptsWithLogger(logging.DefaultLogger(), provider)
}

func BuildClaudeStateMountOptsWithLogger(logger zerolog.Logger, provider env.Provider) []string {
	provider = env.OrDefault(provider)
	home, err := provider.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}

	return buildClaudeStateMountOpts(logger, home)
}

func buildClaudeStateMountOpts(logger zerolog.Logger, home string) []string {
	claudeDir := filepath.Join(home, ".claude")
	claudeJSON := filepath.Join(home, ".claude.json")

	var opts []string
	if st, err := os.Stat(claudeDir); err == nil {
		if st.IsDir() {
			opts = append(opts, "-v "+quote(claudeDir)+":"+quote("/root/.claude")+":rw")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		logger.Warn().Err(err).Str("path", claudeDir).Msg("failed checking Claude state directory; skipping mount")
	}

	if st, err := os.Stat(claudeJSON); err == nil {
		if !st.IsDir() {
			opts = append(opts, "-v "+quote(claudeJSON)+":"+quote("/root/.claude.json")+":rw")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		logger.Warn().Err(err).Str("path", claudeJSON).Msg("failed checking Claude state file; skipping mount")
	}

	if len(opts) == 0 {
		return nil
	}
	return opts
}
