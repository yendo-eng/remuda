package session

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/util"
)

func NewZellijManager() SessionManager {
	return NewZellijManagerWithLogger(logging.DefaultLogger())
}

func NewZellijManagerWithLogger(logger zerolog.Logger) SessionManager {
	return &zellijManager{logger: logger}
}

type zellijManager struct {
	logger zerolog.Logger
}

func (z *zellijManager) SetLogger(logger zerolog.Logger) {
	z.logger = logger
}

func (z *zellijManager) Name() string {
	return string(SessionManagerZellij)
}

func (z *zellijManager) Start(sessionName, command string) error {
	return z.StartWithEnv(sessionName, command, nil)
}

func (z *zellijManager) StartWithEnv(sessionName, command string, env []string) error {
	zellijName := encodeZellijSessionName(sessionName)
	out, err := runZellijCmdCombinedOutput(z.logger, env, "attach", "--create-background", zellijName)
	if err != nil {
		msg := strings.TrimSpace(out)
		if strings.Contains(strings.ToLower(msg), "create-background") {
			return fmt.Errorf("zellij does not support --create-background (requires zellij >= 0.40.0); please upgrade zellij or use --session-manager tmux: %w", err)
		}
		if msg == "" {
			return fmt.Errorf("zellij attach --create-background: %w", err)
		}
		return fmt.Errorf("zellij attach --create-background: %s: %w", msg, err)
	}

	payload := command + "\n"
	deadline := time.Now().Add(2 * time.Second)
	for {
		out, err := runZellijCmdCombinedOutput(z.logger, env, "--session", zellijName, "action", "write-chars", payload)
		if err == nil {
			return nil
		}

		msg := strings.TrimSpace(out)
		if time.Now().After(deadline) || !isRetryableZellijActionError(msg) {
			if msg == "" {
				return fmt.Errorf("zellij write-chars: %w", err)
			}
			return fmt.Errorf("zellij write-chars: %s: %w", msg, err)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func runZellijCmdCombinedOutput(logger zerolog.Logger, env []string, args ...string) (string, error) {
	cmd := util.CmdWithEnvAndLogger(logger, env, "zellij", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

func (z *zellijManager) List() ([]SessionInfo, error) {
	out, err := util.RunCmdCombinedOutputWithLogger(z.logger, "zellij", "list-sessions", "--no-formatting")
	if err != nil {
		// When no server is running, treat as zero sessions.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if strings.Contains(out, "no active") || strings.Contains(out, "No active") {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("zellij list-sessions: %w", err)
	}

	return parseZellijListOutput(out), nil
}

func (z *zellijManager) Find(name string) (SessionInfo, error) {
	sessions, err := z.List()
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

func parseZellijListOutput(s string) []SessionInfo {
	return parseZellijListOutputWithNow(s, time.Now())
}

func parseZellijListOutputWithNow(s string, now time.Time) []SessionInfo {
	now = now.UTC()
	var res []SessionInfo
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		name := line
		if idx := strings.IndexRune(line, '['); idx >= 0 {
			name = strings.TrimSpace(line[:idx])
		}
		if name == "" {
			continue
		}

		exited := strings.Contains(line, "EXITED")
		if exited {
			continue
		}

		attached := strings.Contains(line, "ATTACHED")

		if decoded, ok := decodeZellijSessionName(name); ok {
			name = decoded
		}
		info := SessionInfo{Name: name, Attached: attached}
		if createdAge, ok := parseZellijCreatedAge(line); ok {
			info.CreatedAt = now.Add(-createdAge)
		}
		res = append(res, info)
	}
	return res
}

func parseZellijCreatedAge(line string) (time.Duration, bool) {
	const marker = "Created "
	idx := strings.Index(line, marker)
	if idx < 0 {
		return 0, false
	}
	rest := line[idx+len(marker):]
	end := strings.Index(rest, " ago")
	if end < 0 {
		return 0, false
	}
	ageRaw := strings.TrimSpace(rest[:end])
	if ageRaw == "" {
		return 0, false
	}
	age, ok := parseZellijDurationTokens(ageRaw)
	if !ok {
		return 0, false
	}
	return age, true
}

func parseZellijDurationTokens(raw string) (time.Duration, bool) {
	tokens := strings.Fields(raw)
	if len(tokens) == 0 {
		return 0, false
	}
	var total time.Duration
	for _, tok := range tokens {
		part, ok := parseZellijDurationToken(tok)
		if !ok {
			return 0, false
		}
		total += part
	}
	return total, true
}

func parseZellijDurationToken(token string) (time.Duration, bool) {
	token = strings.TrimSpace(strings.ToLower(token))
	if len(token) < 2 {
		return 0, false
	}
	unit := token[len(token)-1]
	valueRaw := token[:len(token)-1]
	if valueRaw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(valueRaw)
	if err != nil {
		return 0, false
	}
	switch unit {
	case 's':
		return time.Duration(value) * time.Second, true
	case 'm':
		return time.Duration(value) * time.Minute, true
	case 'h':
		return time.Duration(value) * time.Hour, true
	case 'd':
		return time.Duration(value) * 24 * time.Hour, true
	case 'w':
		return time.Duration(value) * 7 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func (z *zellijManager) Attach(name string) error {
	cmd := util.CmdWithLogger(z.logger, "zellij", "attach", encodeZellijSessionName(name))
	cmd.Stdout, cmd.Stdin, cmd.Stderr = os.Stderr, os.Stdin, os.Stderr
	return cmd.Run()
}

func (z *zellijManager) ReadBuffer(name string, lines int) (string, error) {
	if lines < 0 {
		lines = 200
	}

	tmpDir := os.TempDir()
	file, err := os.CreateTemp(tmpDir, "remuda-zellij-*.log")
	if err != nil {
		return "", err
	}
	filename := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}
	defer func() {
		_ = os.Remove(filename)
	}()

	abs, err := filepath.Abs(filename)
	if err != nil {
		abs = filename
	}

	if err := util.RunCmdWithLogger(z.logger, "zellij", "--session", encodeZellijSessionName(name), "action", "dump-screen", abs); err != nil {
		return "", fmt.Errorf("zellij dump-screen: %w", err)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	linesSlice := strings.Split(content, "\n")
	if lines > 0 && len(linesSlice) > lines {
		linesSlice = linesSlice[len(linesSlice)-lines:]
	}

	return strings.Join(linesSlice, "\n"), nil
}

func (z *zellijManager) Send(name string, payload string, appendNewline bool) error {
	out, err := util.RunCmdCombinedOutputWithLogger(z.logger, "zellij", "--session", encodeZellijSessionName(name), "action", "write-chars", payload)
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			return fmt.Errorf("zellij write-chars: %w", err)
		}
		return fmt.Errorf("zellij write-chars: %s: %w", msg, err)
	}

	if appendNewline && !strings.HasSuffix(payload, "\n") && !strings.HasSuffix(payload, "\r") {
		// Codex has a paste burst detector in its TUI; a short delay helps it
		// treat the follow-up Enter as a submit instead of more pasted text.
		time.Sleep(200 * time.Millisecond)
		out, err = util.RunCmdCombinedOutputWithLogger(z.logger, "zellij", "--session", encodeZellijSessionName(name), "action", "write-chars", "\n")
		if err != nil {
			msg := strings.TrimSpace(out)
			if msg == "" {
				return fmt.Errorf("zellij write-chars: %w", err)
			}
			return fmt.Errorf("zellij write-chars: %s: %w", msg, err)
		}
	}
	return nil
}

func (z *zellijManager) Kill(name string) error {
	return util.RunCmdWithLogger(z.logger, "zellij", "delete-session", "--force", encodeZellijSessionName(name))
}

// Zellij session names have a maximum length, and this is what it appears to be
// from testing it on MacOS.
const zellijApparentMaxSessionNameLen = 40

func encodeZellijSessionName(name string) string {
	// Replace slashes with dots and truncate
	encoded := strings.ReplaceAll(name, "/", ".")
	if len(encoded) > zellijApparentMaxSessionNameLen {
		encoded = encoded[:zellijApparentMaxSessionNameLen]
	}
	return encoded
}

func decodeZellijSessionName(name string) (string, bool) {
	// Split the decoded name into org/repo/feature format
	parts := strings.Split(name, ".")
	if len(parts) != 3 {
		return name, false
	}

	decoded := strings.ReplaceAll(name, ".", "/")
	return decoded, true
}

func isRetryableZellijActionError(msg string) bool {
	// Zellij can take a moment to fully register a newly-created background
	// session before it can accept `action` commands.
	msgLower := strings.ToLower(msg)
	switch {
	case strings.Contains(msgLower, "no active sessions"):
		return true
	case strings.Contains(msgLower, "no sessions"):
		return true
	case strings.Contains(msgLower, "session") && strings.Contains(msgLower, "not found"):
		return true
	default:
		return false
	}
}
