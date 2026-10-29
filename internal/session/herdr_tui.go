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
	herdrTUIMetadataSource          = "remuda"
	herdrTUIAgentStartRetryInterval = 100 * time.Millisecond
	herdrTUIAgentStartRetryTimeout  = 2 * time.Second
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

type herdrTUI struct {
	logger zerolog.Logger
}

type herdrTUIResponse[T any] struct {
	Result T `json:"result"`
}

type herdrTUICommandError struct {
	code   string
	output string
	err    error
}

func (e *herdrTUICommandError) Error() string {
	return fmt.Sprintf("herdr: %s: %v", e.output, e.err)
}

func (e *herdrTUICommandError) Unwrap() error {
	return e.err
}

type herdrTUIWorkspace struct {
	WorkspaceID string            `json:"workspace_id"`
	Tokens      map[string]string `json:"tokens"`
}

type herdrTUIPane struct {
	PaneID string `json:"pane_id"`
}

type herdrTUITab struct {
	TabID string `json:"tab_id"`
}

func NewHerdrTUI() Multiplexer {
	return NewHerdrTUIWithLogger(logging.DefaultLogger())
}

func NewHerdrTUIWithLogger(logger zerolog.Logger) Multiplexer {
	return &herdrTUI{logger: logger}
}

func (h *herdrTUI) SetLogger(logger zerolog.Logger) {
	h.logger = logger
}

func (h *herdrTUI) Name() string {
	return string(MultiplexerHerdrTUI)
}

func (h *herdrTUI) Start(_, _ string) error {
	return pkgerrors.New("herdr requires a structured agent start")
}

func (h *herdrTUI) StartAgent(start AgentStart) error {
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

	var created herdrTUIResponse[struct {
		Workspace herdrTUIWorkspace `json:"workspace"`
		Tab       herdrTUITab       `json:"tab"`
		RootPane  herdrTUIPane      `json:"root_pane"`
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
		"--source", herdrTUIMetadataSource,
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

	agentHandle := herdrTUIAgentHandle(start.SessionName)
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
func (h *herdrTUI) startContainerInPane(paneID string, command []string) error {
	if len(command) == 0 {
		return pkgerrors.New("herdr container launch is missing command argv")
	}

	commandPath, err := writeHerdrTUILaunchScript(command)
	if err != nil {
		return err
	}
	if _, err := h.runOutput("pane", "run", paneID, commandPath); err != nil {
		_ = os.Remove(commandPath)
		return err
	}
	return nil
}

func writeHerdrTUILaunchScript(command []string) (string, error) {
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

func (h *herdrTUI) startAgent(args []string) error {
	deadline := time.Now().Add(herdrTUIAgentStartRetryTimeout)
	for {
		_, err := h.runOutput(args...)
		if err == nil {
			return nil
		}
		var commandErr *herdrTUICommandError
		if !errors.As(err, &commandErr) || commandErr.code != "agent_pane_busy" || !time.Now().Before(deadline) {
			return err
		}
		time.Sleep(herdrTUIAgentStartRetryInterval)
	}
}

func (h *herdrTUI) List() ([]SessionInfo, error) {
	workspaces, err := h.workspaces()
	if err != nil {
		return nil, err
	}

	sessions := make([]SessionInfo, 0, len(workspaces))
	for _, workspace := range workspaces {
		if workspace.Tokens["remuda"] != "1" {
			continue
		}
		name, ok := herdrTUIWorkspaceSessionName(workspace)
		if !ok {
			h.logger.Debug().Str("workspace", workspace.WorkspaceID).Msg("skipping herdr workspace with invalid remuda identity metadata")
			continue
		}
		info := SessionInfo{Name: name, Multiplexer: string(MultiplexerHerdrTUI)}
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

func (h *herdrTUI) Find(name string) (SessionInfo, error) {
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

func (h *herdrTUI) Attach(name string) error {
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

func (h *herdrTUI) ReadBuffer(name string, lines int) (string, error) {
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

func (h *herdrTUI) Send(name, payload string, appendNewline bool) error {
	pane, err := h.primaryPane(name)
	if err != nil {
		return err
	}
	return h.sendToPane(pane.PaneID, payload, appendNewline)
}

func (h *herdrTUI) sendToPane(paneID, payload string, appendNewline bool) error {
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

func (h *herdrTUI) Kill(name string) error {
	workspace, err := h.findWorkspace(name)
	if err != nil {
		return err
	}
	_, err = h.runOutput("workspace", "close", workspace.WorkspaceID)
	return err
}

func (h *herdrTUI) primaryPane(name string) (herdrTUIPane, error) {
	workspace, err := h.findWorkspace(name)
	if err != nil {
		return herdrTUIPane{}, err
	}
	output, err := h.runOutput("pane", "list", "--workspace", workspace.WorkspaceID)
	if err != nil {
		return herdrTUIPane{}, err
	}
	var response herdrTUIResponse[struct {
		Panes []herdrTUIPane `json:"panes"`
	}]
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return herdrTUIPane{}, pkgerrors.Wrap(err, "decode herdr pane list response")
	}
	if len(response.Result.Panes) == 0 {
		return herdrTUIPane{}, pkgerrors.Errorf("herdr workspace %s has no panes", workspace.WorkspaceID)
	}
	return response.Result.Panes[0], nil
}

func (h *herdrTUI) findWorkspace(name string) (herdrTUIWorkspace, error) {
	workspaces, err := h.workspaces()
	if err != nil {
		return herdrTUIWorkspace{}, err
	}
	for _, workspace := range workspaces {
		if workspaceName, ok := herdrTUIWorkspaceSessionName(workspace); ok && workspaceName == name {
			return workspace, nil
		}
	}
	return herdrTUIWorkspace{}, ErrSessionNotFound
}

func (h *herdrTUI) closeWorkspaceBestEffort(workspaceID string) {
	if _, err := h.runOutput("workspace", "close", workspaceID); err != nil {
		h.logger.Debug().Err(err).Str("workspace", workspaceID).Msg("failed to clean up herdr workspace")
	}
}

func (h *herdrTUI) workspaces() ([]herdrTUIWorkspace, error) {
	output, err := h.runOutput("workspace", "list")
	if err != nil {
		return nil, err
	}
	var response herdrTUIResponse[struct {
		Workspaces []herdrTUIWorkspace `json:"workspaces"`
	}]
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return nil, pkgerrors.Wrap(err, "decode herdr workspace list response")
	}
	return response.Result.Workspaces, nil
}

func (h *herdrTUI) runOutput(args ...string) (string, error) {
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
			return "", &herdrTUICommandError{
				code:   response.Error.Code,
				output: message,
				err:    err,
			}
		}
		return "", pkgerrors.Wrapf(err, "herdr: %s", message)
	}
	return string(output), nil
}

func (h *herdrTUI) command(args ...string) *exec.Cmd {
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

func herdrTUIWorkspaceSessionName(workspace herdrTUIWorkspace) (string, bool) {
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

func herdrTUIAgentHandle(sessionName string) string {
	sum := sha256.Sum256([]byte(sessionName))
	return fmt.Sprintf("remuda-%s", hex.EncodeToString(sum[:])[:12])
}
