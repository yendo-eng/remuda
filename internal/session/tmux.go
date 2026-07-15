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

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/util"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
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
	envFile, err := writeTmuxEnvFile(env)
	if err != nil {
		return err
	}

	args := []string{"new-session", "-d", "-s", sessionName}
	if envFile != "" {
		command = tmuxCommandWithEnvFile(envFile, command)
	}
	args = append(args, "bash", "-lc", command)
	if err := util.RunCmdWithEnvAndLogger(m.logger, env, "tmux", args...); err != nil {
		if envFile != "" {
			_ = os.Remove(envFile)
		}
		return pkgerrors.Wrapf(err, "tmux new-session %s", sessionName)
	}
	return nil
}

func writeTmuxEnvFile(env []string) (string, error) {
	contents := tmuxEnvFileContents(env)
	if contents == "" {
		return "", nil
	}

	file, err := os.CreateTemp("", "remuda-tmux-env-")
	if err != nil {
		return "", pkgerrors.Wrap(err, "create tmux environment file")
	}
	path := file.Name()
	removeFile := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if err := file.Chmod(0o600); err != nil {
		removeFile()
		return "", pkgerrors.Wrap(err, "set tmux environment file permissions")
	}
	if _, err := file.WriteString(contents); err != nil {
		removeFile()
		return "", pkgerrors.Wrap(err, "write tmux environment file")
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", pkgerrors.Wrap(err, "close tmux environment file")
	}
	return path, nil
}

func tmuxEnvFileContents(env []string) string {
	var contents strings.Builder
	for _, kv := range env {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		if !util.IsValidEnvVarName(key) || key == "TMUX" || key == "TMUX_PANE" {
			continue
		}
		contents.WriteString("export ")
		contents.WriteString(key)
		contents.WriteByte('=')
		contents.WriteString(shellutil.SingleQuote(value))
		contents.WriteByte('\n')
	}
	return contents.String()
}

func tmuxCommandWithEnvFile(envFile, command string) string {
	quotedFile := shellutil.SingleQuote(envFile)
	cleanup := "rm -f -- " + quotedFile
	inner := strings.Join([]string{
		"trap " + shellutil.SingleQuote(cleanup) + " EXIT",
		". " + quotedFile,
		cleanup,
		command,
	}, "; ")
	return "exec env -i TERM=\"$TERM\" TMUX=\"$TMUX\" TMUX_PANE=\"$TMUX_PANE\" USER=\"$USER\" LOGNAME=\"$LOGNAME\" bash -lc " + shellutil.SingleQuote(inner)
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
		return nil, pkgerrors.Wrap(err, "tmux list-sessions")
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
		return "", pkgerrors.Wrap(err, "tmux capture-pane")
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
			return pkgerrors.Wrap(err, "tmux send-keys")
		}
	}

	if appendNewline && !strings.HasSuffix(payload, "\n") && !strings.HasSuffix(payload, "\r") {
		// Codex has a paste burst detector in its TUI; a short delay helps it
		// treat the follow-up Enter as a submit instead of more pasted text.
		time.Sleep(200 * time.Millisecond)
		if err := util.RunCmdWithLogger(m.logger, "tmux", "send-keys", "-t", target, "Enter"); err != nil {
			return pkgerrors.Wrap(err, "tmux send-keys")
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
