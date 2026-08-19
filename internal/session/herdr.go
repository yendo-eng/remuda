package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

const (
	herdrMetadataSource          = "remuda"
	herdrAgentStartRetryInterval = 100 * time.Millisecond
	herdrAgentStartRetryTimeout  = 2 * time.Second
)

type UnsupportedAgentCommandError struct{}

func (UnsupportedAgentCommandError) Error() string {
	return "--agent-cmd is not supported by the herdr session manager"
}

type SessionAlreadyExistsError struct {
	Name string
}

func (e SessionAlreadyExistsError) Error() string {
	return fmt.Sprintf("session %q already exists", e.Name)
}

type herdr struct {
	logger zerolog.Logger
}

type herdrResponse[T any] struct {
	Result T `json:"result"`
}

type herdrCommandError struct {
	code   string
	output string
	err    error
}

func (e *herdrCommandError) Error() string {
	return fmt.Sprintf("herdr: %s: %v", e.output, e.err)
}

func (e *herdrCommandError) Unwrap() error {
	return e.err
}

type herdrWorkspace struct {
	WorkspaceID string            `json:"workspace_id"`
	Tokens      map[string]string `json:"tokens"`
}

type herdrPane struct {
	PaneID string `json:"pane_id"`
}

type herdrTab struct {
	TabID string `json:"tab_id"`
}

func NewHerdr() Multiplexer {
	return NewHerdrWithLogger(logging.DefaultLogger())
}

func NewHerdrWithLogger(logger zerolog.Logger) Multiplexer {
	return &herdr{logger: logger}
}

func (h *herdr) SetLogger(logger zerolog.Logger) {
	h.logger = logger
}

func (h *herdr) Name() string {
	return string(MultiplexerHerdr)
}

func (h *herdr) Start(_, _ string) error {
	return pkgerrors.New("herdr requires a structured agent start")
}

func (h *herdr) StartAgent(start AgentStart) error {
	if start.Agent == "custom" {
		return UnsupportedAgentCommandError{}
	}

	info := SessionInfo{Name: start.SessionName}
	if !info.IsRemudaSession() {
		return pkgerrors.New("invalid session name format")
	}
	if _, err := h.findWorkspace(start.SessionName); err == nil {
		return SessionAlreadyExistsError{Name: start.SessionName}
	} else if !errors.Is(err, ErrSessionNotFound) {
		return err
	}

	parts := strings.Split(start.SessionName, "/")

	args := []string{"workspace", "create", "--cwd", start.Workspace, "--label", start.SessionName, "--no-focus"}
	// Herdr has no env-file or stdin channel. These values remain visible in
	// process listings while the short-lived workspace-create CLI command runs.
	for _, value := range start.Env {
		if _, _, ok := strings.Cut(value, "="); ok {
			args = append(args, "--env", value)
		}
	}
	if start.Container {
		args = append(args, "--env", "HERDR_AGENT="+start.Agent)
	}
	output, err := h.runOutput(args...)
	if err != nil {
		return err
	}

	var created herdrResponse[struct {
		Workspace herdrWorkspace `json:"workspace"`
		Tab       herdrTab       `json:"tab"`
		RootPane  herdrPane      `json:"root_pane"`
	}]
	if err := json.Unmarshal([]byte(output), &created); err != nil {
		return pkgerrors.Wrap(err, "decode herdr workspace create response")
	}
	if created.Result.Workspace.WorkspaceID == "" {
		return pkgerrors.New("herdr workspace create response is missing workspace or pane id")
	}
	started := false
	defer func() {
		if !started {
			h.closeWorkspaceBestEffort(created.Result.Workspace.WorkspaceID)
		}
	}()
	if created.Result.RootPane.PaneID == "" {
		return pkgerrors.New("herdr workspace create response is missing workspace or pane id")
	}
	if created.Result.Tab.TabID == "" {
		return pkgerrors.New("herdr workspace create response is missing tab id")
	}
	if _, err := h.runOutput("tab", "rename", created.Result.Tab.TabID, "agent"); err != nil {
		return err
	}

	metadataArgs := []string{
		"workspace", "report-metadata", created.Result.Workspace.WorkspaceID,
		"--source", herdrMetadataSource,
		"--token", "remuda=1",
		"--token", "org=" + parts[0],
		"--token", "repo=" + parts[1],
		"--token", "folder=" + parts[2],
		"--token", "created_at=" + time.Now().UTC().Format(time.RFC3339Nano),
	}
	if _, err := h.runOutput(metadataArgs...); err != nil {
		return err
	}

	if start.Container {
		if err := h.startContainerInPane(created.Result.RootPane.PaneID, start.CommandArgv); err != nil {
			return err
		}
		started = true
		return nil
	}

	if start.Agent == "bash" {
		if len(start.Args) > 0 {
			h.logger.Debug().
				Str("session", start.SessionName).
				Int("argument_count", len(start.Args)).
				Msg("bash uses the herdr root shell; agent arguments are ignored")
		}
		started = true
		return nil
	}

	agentHandle := herdrAgentHandle(start.SessionName)
	agentArgs := []string{
		"agent", "start", agentHandle,
		"--kind", start.Agent,
		"--pane", created.Result.RootPane.PaneID,
		"--",
	}
	agentArgs = append(agentArgs, start.Args...)
	if err := h.startAgent(agentArgs); err != nil {
		return err
	}
	if start.Prompt != "" {
		if _, err := h.runOutput("agent", "prompt", agentHandle, start.Prompt); err != nil {
			return err
		}
	}
	started = true
	return nil
}

// Herdr's root pane is an interactive shell, so it survives the container
// process exiting. The script keeps Docker as the foreground process for
// HERDR_AGENT detection while handing pane ownership back to that shell.
func (h *herdr) startContainerInPane(paneID string, command []string) error {
	if len(command) == 0 {
		return pkgerrors.New("herdr container launch is missing command argv")
	}

	commandPath, err := writeHerdrLaunchScript(command)
	if err != nil {
		return err
	}
	if _, err := h.runOutput("pane", "run", paneID, commandPath); err != nil {
		_ = os.Remove(commandPath)
		return err
	}
	return nil
}

func writeHerdrLaunchScript(command []string) (string, error) {
	quoted := make([]string, len(command))
	for i, arg := range command {
		quoted[i] = shellutil.SingleQuote(arg)
	}

	file, err := os.CreateTemp("", "remuda-herdr-*.sh")
	if err != nil {
		return "", pkgerrors.Wrap(err, "create Herdr container launch script")
	}
	path := file.Name()
	removeOnError := true
	defer func() {
		if removeOnError {
			_ = os.Remove(path)
		}
	}()

	script := "#!/bin/sh\nrm -f -- \"$0\"\nexec " + strings.Join(quoted, " ") + "\n"
	if _, err := file.WriteString(script); err != nil {
		_ = file.Close()
		return "", pkgerrors.Wrap(err, "write Herdr container launch script")
	}
	if err := file.Chmod(0o700); err != nil {
		_ = file.Close()
		return "", pkgerrors.Wrap(err, "make Herdr container launch script executable")
	}
	if err := file.Close(); err != nil {
		return "", pkgerrors.Wrap(err, "close Herdr container launch script")
	}
	removeOnError = false
	return path, nil
}

func (h *herdr) startAgent(args []string) error {
	deadline := time.Now().Add(herdrAgentStartRetryTimeout)
	for {
		_, err := h.runOutput(args...)
		if err == nil {
			return nil
		}
		var commandErr *herdrCommandError
		if !errors.As(err, &commandErr) || commandErr.code != "agent_pane_busy" || !time.Now().Before(deadline) {
			return err
		}
		time.Sleep(herdrAgentStartRetryInterval)
	}
}

func (h *herdr) List() ([]SessionInfo, error) {
	workspaces, err := h.workspaces()
	if err != nil {
		return nil, err
	}

	sessions := make([]SessionInfo, 0, len(workspaces))
	for _, workspace := range workspaces {
		if workspace.Tokens["remuda"] != "1" {
			continue
		}
		name, ok := herdrWorkspaceSessionName(workspace)
		if !ok {
			h.logger.Debug().Str("workspace", workspace.WorkspaceID).Msg("skipping herdr workspace with invalid remuda identity metadata")
			continue
		}
		info := SessionInfo{Name: name, Multiplexer: string(MultiplexerHerdr)}
		createdAt, err := time.Parse(time.RFC3339Nano, workspace.Tokens["created_at"])
		if err == nil {
			info.CreatedAt = createdAt
		} else {
			h.logger.Debug().Str("workspace", workspace.WorkspaceID).Msg("herdr workspace has invalid created_at metadata")
		}
		sessions = append(sessions, info)
	}
	return sessions, nil
}

func (h *herdr) Find(name string) (SessionInfo, error) {
	sessions, err := h.List()
	if err != nil {
		return SessionInfo{}, err
	}
	for _, info := range sessions {
		if info.Name == name {
			return info, nil
		}
	}
	return SessionInfo{}, ErrSessionNotFound
}

func (h *herdr) Attach(name string) error {
	workspace, err := h.findWorkspace(name)
	if err != nil {
		return err
	}
	if _, err := h.runOutput("workspace", "focus", workspace.WorkspaceID); err != nil {
		return err
	}

	cmd := h.command()
	cmd.Stdout, cmd.Stdin, cmd.Stderr = os.Stderr, os.Stdin, os.Stderr
	return cmd.Run()
}

func (h *herdr) ReadBuffer(name string, lines int) (string, error) {
	pane, err := h.primaryPane(name)
	if err != nil {
		return "", err
	}
	if lines < 0 {
		lines = 200
	}

	args := []string{"pane", "read", pane.PaneID, "--source", "recent"}
	if lines > 0 {
		args = append(args, "--lines", strconv.Itoa(lines))
	}
	return h.runOutput(args...)
}

func (h *herdr) Send(name, payload string, appendNewline bool) error {
	pane, err := h.primaryPane(name)
	if err != nil {
		return err
	}
	return h.sendToPane(pane.PaneID, payload, appendNewline)
}

func (h *herdr) sendToPane(paneID, payload string, appendNewline bool) error {
	if payload != "" {
		if _, err := h.runOutput("pane", "send-text", paneID, payload); err != nil {
			return err
		}
	}
	if appendNewline && !strings.HasSuffix(payload, "\n") && !strings.HasSuffix(payload, "\r") {
		// Bash root panes have no managed agent target for agent prompt. Keep the
		// pane path and delay Enter so Codex does not treat it as pasted text.
		time.Sleep(200 * time.Millisecond)
		_, err := h.runOutput("pane", "send-keys", paneID, "enter")
		return err
	}
	return nil
}

func (h *herdr) Kill(name string) error {
	workspace, err := h.findWorkspace(name)
	if err != nil {
		return err
	}
	_, err = h.runOutput("workspace", "close", workspace.WorkspaceID)
	return err
}

func (h *herdr) primaryPane(name string) (herdrPane, error) {
	workspace, err := h.findWorkspace(name)
	if err != nil {
		return herdrPane{}, err
	}
	output, err := h.runOutput("pane", "list", "--workspace", workspace.WorkspaceID)
	if err != nil {
		return herdrPane{}, err
	}
	var response herdrResponse[struct {
		Panes []herdrPane `json:"panes"`
	}]
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return herdrPane{}, pkgerrors.Wrap(err, "decode herdr pane list response")
	}
	if len(response.Result.Panes) == 0 {
		return herdrPane{}, pkgerrors.Errorf("herdr workspace %s has no panes", workspace.WorkspaceID)
	}
	return response.Result.Panes[0], nil
}

func (h *herdr) findWorkspace(name string) (herdrWorkspace, error) {
	workspaces, err := h.workspaces()
	if err != nil {
		return herdrWorkspace{}, err
	}
	for _, workspace := range workspaces {
		if workspaceName, ok := herdrWorkspaceSessionName(workspace); ok && workspaceName == name {
			return workspace, nil
		}
	}
	return herdrWorkspace{}, ErrSessionNotFound
}

func (h *herdr) closeWorkspaceBestEffort(workspaceID string) {
	if _, err := h.runOutput("workspace", "close", workspaceID); err != nil {
		h.logger.Debug().Err(err).Str("workspace", workspaceID).Msg("failed to clean up herdr workspace")
	}
}

func (h *herdr) workspaces() ([]herdrWorkspace, error) {
	output, err := h.runOutput("workspace", "list")
	if err != nil {
		return nil, err
	}
	var response herdrResponse[struct {
		Workspaces []herdrWorkspace `json:"workspaces"`
	}]
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return nil, pkgerrors.Wrap(err, "decode herdr workspace list response")
	}
	return response.Result.Workspaces, nil
}

func (h *herdr) runOutput(args ...string) (string, error) {
	cmd := h.command(args...)
	output, err := cmd.Output()
	if err != nil {
		message := ""
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			message = strings.TrimSpace(string(exitErr.Stderr))
		}
		if message == "" {
			return "", pkgerrors.Wrap(err, "herdr")
		}
		var response struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(message), &response) == nil && response.Error.Code != "" {
			return "", &herdrCommandError{
				code:   response.Error.Code,
				output: message,
				err:    err,
			}
		}
		return "", pkgerrors.Wrapf(err, "herdr: %s", message)
	}
	return string(output), nil
}

func (h *herdr) command(args ...string) *exec.Cmd {
	// Log only the command family: workspace create argv can contain secrets.
	command := "herdr"
	if len(args) > 0 {
		command += " " + args[0]
	}
	if len(args) > 1 {
		command += " " + args[1]
	}
	h.logger.Debug().Str("cmd", command).Msg("command")
	//nolint:gosec,noctx // G204: args are passed as argv to the selected herdr backend.
	return exec.Command("herdr", args...)
}

func herdrWorkspaceSessionName(workspace herdrWorkspace) (string, bool) {
	if workspace.Tokens["remuda"] != "1" {
		return "", false
	}
	name := strings.Join([]string{
		workspace.Tokens["org"],
		workspace.Tokens["repo"],
		workspace.Tokens["folder"],
	}, "/")
	return name, (SessionInfo{Name: name}).IsRemudaSession()
}

func herdrAgentHandle(sessionName string) string {
	sum := sha256.Sum256([]byte(sessionName))
	return fmt.Sprintf("remuda-%s", hex.EncodeToString(sum[:])[:12])
}
