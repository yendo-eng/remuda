package session

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/util"
)

func NewTmuxManager() SessionManager {
	return NewTmuxManagerWithLogger(logging.DefaultLogger())
}

// defaultTmuxManager is the production implementation backed by the `tmux` CLI.
type defaultTmuxManager struct {
	logger zerolog.Logger
}

func NewTmuxManagerWithLogger(logger zerolog.Logger) SessionManager {
	return &defaultTmuxManager{logger: logger}
}

func (m *defaultTmuxManager) SetLogger(logger zerolog.Logger) {
	m.logger = logger
}

func (m *defaultTmuxManager) Name() string {
	return string(SessionManagerTmux)
}

func (m *defaultTmuxManager) Start(sessionName, command string) error {
	return m.StartWithEnv(sessionName, command, nil)
}

func (m *defaultTmuxManager) StartWithEnv(sessionName, command string, env []string) error {
	args := []string{"new-session", "-d", "-s", sessionName}
	args = append(args, tmuxNewSessionEnvArgs(env)...)
	args = append(args, "bash", "-lc", command)
	return util.RunCmdWithEnvAndLogger(m.logger, env, "tmux", args...)
}

func tmuxNewSessionEnvArgs(env []string) []string {
	args := make([]string, 0, len(env)*2)
	for _, kv := range env {
		key, _, ok := strings.Cut(kv, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		if !util.IsValidEnvVarName(key) || key == "TMUX" || key == "TMUX_PANE" {
			continue
		}
		args = append(args, "-e", kv)
	}
	return args
}

func (m *defaultTmuxManager) resolveSessionName(name string) (string, error) {
	target := strings.TrimSpace(name)
	if target == "" {
		return "", ErrSessionNotFound
	}
	sessions, err := m.List()
	if err != nil {
		return "", err
	}
	sanitizedTarget := sanitizeTmuxSessionToken(target)
	for _, s := range sessions {
		if s.Name == target {
			return s.Name, nil
		}
	}
	for _, s := range sessions {
		if s.Name == sanitizedTarget {
			return s.Name, nil
		}
	}
	for _, s := range sessions {
		if sanitizeTmuxSessionToken(s.Name) == sanitizedTarget {
			return s.Name, nil
		}
	}
	return "", ErrSessionNotFound
}

func (m *defaultTmuxManager) List() ([]SessionInfo, error) {
	// -F format: name SPACE attached(1/0) SPACE created(epoch seconds)
	// We assume tmux always expands #{session_created} to a non-empty token.
	// If that token were missing, names with spaces could be mis-parsed.
	// Use CombinedOutput so we can inspect stderr when tmux returns
	// exit code 1 for "no server running" (no sessions). Treat that
	// condition as an empty list rather than an error.
	//
	// NOTE: We intentionally avoid a literal TAB delimiter here. Some tmux builds
	// appear to sanitize control characters in this output (rendering TAB as "_"),
	// which breaks parsing and causes Remuda to incorrectly report "no sessions".
	out, err := util.RunCmdCombinedOutputWithLogger(m.logger, "tmux", "list-sessions", "-F", "#{session_name} #{?session_attached,1,0} #{session_created}")
	if err != nil {
		// If tmux exits with status 1 (common when no server is running),
		// interpret that as "no sessions" regardless of stderr wording.
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return nil, nil
		}
		// Fallback: also handle well-known messages even if exit code isn't available.
		if strings.Contains(out, "no server running") || strings.Contains(out, "no sessions") {
			return nil, nil
		}
		return nil, fmt.Errorf("tmux list-sessions: %w", err)
	}

	return parseTmuxListOutput(string(out)), nil
}

func (m *defaultTmuxManager) Find(name string) (SessionInfo, error) {
	sessions, err := m.List()
	if err != nil {
		return SessionInfo{}, err
	}
	for _, s := range sessions {
		if s.Name == name {
			return s, nil
		}
	}
	return SessionInfo{}, ErrSessionNotFound
}

// parseTmuxListOutput converts lines of "<name> <0|1> [<epoch>]" into SessionInfo.
func parseTmuxListOutput(s string) []SessionInfo {
	var res []SessionInfo
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		// Split from the end so session names with spaces still work.
		lastSpace := strings.LastIndexByte(line, ' ')
		if lastSpace <= 0 || lastSpace >= len(line)-1 {
			continue
		}

		head := strings.TrimSpace(line[:lastSpace])
		tail := strings.TrimSpace(line[lastSpace+1:])
		if head == "" || tail == "" {
			continue
		}

		secondSpace := strings.LastIndexByte(head, ' ')
		var name, attachedRaw, createdRaw string
		if secondSpace == -1 {
			name = strings.TrimSpace(head)
			attachedRaw = tail
		} else {
			name = strings.TrimSpace(head[:secondSpace])
			attachedRaw = strings.TrimSpace(head[secondSpace+1:])
			createdRaw = tail
		}
		if name == "" {
			continue
		}
		if attachedRaw != "0" && attachedRaw != "1" {
			continue
		}
		attached := attachedRaw == "1"
		info := SessionInfo{Name: name, Attached: attached}
		if createdRaw != "" {
			if epoch, err := strconv.ParseInt(createdRaw, 10, 64); err == nil {
				info.CreatedAt = time.Unix(epoch, 0).UTC()
			}
		}
		res = append(res, info)
	}
	return res
}

func (m *defaultTmuxManager) Attach(name string) error {
	resolved, err := m.resolveSessionName(name)
	if err != nil {
		return err
	}
	// tmux uses '.' as a pane separator in target specs; appending ':' forces
	// interpretation as a session target even when the session name contains dots.
	target := resolved + ":"

	cmd := util.CmdWithLogger(m.logger, "tmux", "attach", "-t", target)
	// Ensures the tmux session has access to your terminal.
	cmd.Stdout, cmd.Stdin, cmd.Stderr = os.Stderr, os.Stdin, os.Stderr
	return cmd.Run()
}

func (m *defaultTmuxManager) ReadBuffer(name string, lines int) (string, error) {
	resolved, err := m.resolveSessionName(name)
	if err != nil {
		return "", err
	}
	if lines < 0 {
		lines = 200
	}

	// Capture from the first pane of the first window.
	target := fmt.Sprintf("%s:0.0", resolved)
	out, err := util.RunCmdOutputWithLogger(m.logger, "tmux", "capture-pane", "-p", "-S", "-", "-t", target)
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane: %w", err)
	}

	content := strings.ReplaceAll(string(out), "\r\n", "\n")
	linesSlice := strings.Split(content, "\n")
	if lines > 0 {
		for len(linesSlice) > 0 && strings.TrimSpace(linesSlice[len(linesSlice)-1]) == "" {
			linesSlice = linesSlice[:len(linesSlice)-1]
		}
		if len(linesSlice) > lines {
			linesSlice = linesSlice[len(linesSlice)-lines:]
		}
	}

	return strings.Join(linesSlice, "\n"), nil
}

func (m *defaultTmuxManager) Send(name string, payload string, appendNewline bool) error {
	resolved, err := m.resolveSessionName(name)
	if err != nil {
		return err
	}
	target := resolved + ":"

	if payload != "" {
		if err := util.RunCmdWithLogger(m.logger, "tmux", "send-keys", "-t", target, "-l", payload); err != nil {
			return fmt.Errorf("tmux send-keys: %w", err)
		}
	}

	if appendNewline && !strings.HasSuffix(payload, "\n") && !strings.HasSuffix(payload, "\r") {
		// Codex has a paste burst detector in its TUI; a short delay helps it
		// treat the follow-up Enter as a submit instead of more pasted text.
		time.Sleep(200 * time.Millisecond)
		if err := util.RunCmdWithLogger(m.logger, "tmux", "send-keys", "-t", target, "Enter"); err != nil {
			return fmt.Errorf("tmux send-keys: %w", err)
		}
	}

	return nil
}

func (m *defaultTmuxManager) Kill(name string) error {
	resolved, err := m.resolveSessionName(name)
	if err != nil {
		return err
	}
	target := resolved + ":"
	return util.RunCmdWithLogger(m.logger, "tmux", "kill-session", "-t", target)
}
