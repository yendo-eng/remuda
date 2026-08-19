package session_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/session"
)

func TestHerdrStartAgent(t *testing.T) {
	tests := []struct {
		name      string
		agent     string
		wantStart bool
	}{
		{name: "managed agent", agent: "codex", wantStart: true},
		{name: "bash root pane", agent: "bash", wantStart: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "calls")
			writeHerdrStub(t, dir, `
case "$1 $2" in
  "workspace list")
    printf '%s\n' '{"result":{"workspaces":[]}}'
    ;;
  "workspace create")
    printf '%s\n' '{"result":{"workspace":{"workspace_id":"w7"},"root_pane":{"pane_id":"w7:p1"}}}'
    ;;
esac
`)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("REMUDA_HERDR_CALLS", logPath)

			var logs bytes.Buffer
			starter, ok := session.NewHerdrWithLogger(zerolog.New(&logs).Level(zerolog.DebugLevel)).(session.AgentStarter)
			require.True(t, ok)
			err := starter.StartAgent(session.AgentStart{
				SessionName: "yendo/remuda/rm-ypr2",
				Workspace:   "/workspaces/rm-ypr2",
				Agent:       tt.agent,
				Args:        []string{"--model", "gpt-5.5", "fix it"},
				Prompt:      "finish the work",
				Env:         []string{"PATH=/usr/bin", "OPENAI_API_KEY=secret"},
			})
			require.NoError(t, err)
			require.NotContains(t, logs.String(), "secret")
			require.NotContains(t, logs.String(), "fix it")
			if tt.agent == "bash" {
				require.Contains(t, logs.String(), "agent arguments are ignored")
			}

			calls := readHerdrCalls(t, logPath)
			require.Equal(t, "workspace list", calls[0])
			require.Contains(t, calls[1], "workspace create --cwd /workspaces/rm-ypr2 --label yendo/remuda/rm-ypr2 --no-focus")
			require.Contains(t, calls[1], "--env PATH=/usr/bin --env OPENAI_API_KEY=secret")
			require.Contains(t, calls[2], "workspace report-metadata w7 --source remuda")
			require.Contains(t, calls[2], "--token remuda=1")
			require.Contains(t, calls[2], "--token org=yendo")
			require.Contains(t, calls[2], "--token repo=remuda")
			require.Contains(t, calls[2], "--token folder=rm-ypr2")
			require.Contains(t, calls[2], "--token created_at=")
			if tt.wantStart {
				require.Len(t, calls, 5)
				require.Contains(t, calls[3], "agent start remuda-")
				require.Contains(t, calls[3], "--kind codex --pane w7:p1 -- --model gpt-5.5 fix it")
				require.NotContains(t, calls[3], "finish")
				require.Contains(t, calls[4], "agent prompt remuda-")
			} else {
				require.Len(t, calls, 3)
			}
		})
	}
}

func TestHerdrStartAgentRunsLongContainerArgvInPersistentPane(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a Unix shell stub")
	}

	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	scriptPath := filepath.Join(dir, "launch-script")
	writeHerdrStub(t, dir, fmt.Sprintf(`
case "$1 $2" in
  "workspace list")
    printf '%%s\n' '{"result":{"workspaces":[]}}'
    ;;
  "workspace create")
    printf '%%s\n' '{"result":{"workspace":{"workspace_id":"w7"},"tab":{"tab_id":"w7:t1"},"root_pane":{"pane_id":"w7:p1"}}}'
    ;;
  "pane run")
    cat "$4" > "$REMUDA_HERDR_SCRIPT"
    ;;
esac
`))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("REMUDA_HERDR_CALLS", logPath)
	t.Setenv("REMUDA_HERDR_SCRIPT", scriptPath)

	command := []string{"docker", "run", "--rm", "-it"}
	for i := 0; i < 80; i++ {
		command = append(command, "-v", fmt.Sprintf(
			"/Users/devin/Library/Application Support/remuda/mount-%02d:/workspaces/mount-%02d:ro",
			i, i,
		))
	}
	command = append(command, "remuda-agent:latest", "bash", "-lc", "exec codex")
	require.Greater(t, len(strings.Join(command, " ")), 1500)

	starter, ok := session.NewHerdr().(session.AgentStarter)
	require.True(t, ok)
	require.NoError(t, starter.StartAgent(session.AgentStart{
		SessionName: "yendo/remuda/rm-f20z",
		Workspace:   "/workspaces/rm-f20z",
		Agent:       "codex",
		Command:     strings.Join(command, " "),
		CommandArgv: command,
		Container:   true,
		Env:         []string{"PATH=/usr/bin"},
	}))

	calls := readHerdrCalls(t, logPath)
	require.Contains(t, calls[1], "--env HERDR_AGENT=codex")
	require.Contains(t, calls[2], "workspace report-metadata w7 --source remuda")
	require.Contains(t, calls[3], "pane run w7:p1")
	require.NotContains(t, strings.Join(calls, "\n"), "agent start")

	script, err := os.ReadFile(scriptPath)
	require.NoError(t, err)
	require.Contains(t, string(script), `rm -f -- "$0"`)
	require.Contains(t, string(script), "exec 'docker' 'run' '--rm' '-it'")
	require.Contains(t, string(script), "'mount-00:/workspaces/mount-00:ro'")
	require.Contains(t, string(script), "'exec codex'")
}

func TestHerdrStartAgentRetriesWhilePaneShellInitializes(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	busyMarker := filepath.Join(dir, "busy")
	writeHerdrStub(t, dir, `
case "$1 $2" in
  "workspace list")
    printf '%s\n' '{"result":{"workspaces":[]}}'
    ;;
  "workspace create")
    printf '%s\n' '{"result":{"workspace":{"workspace_id":"w7"},"root_pane":{"pane_id":"w7:p1"}}}'
    ;;
  "agent start")
    if [ ! -e "$REMUDA_HERDR_BUSY_MARKER" ]; then
      : > "$REMUDA_HERDR_BUSY_MARKER"
      printf '%s\n' '{"error":{"code":"agent_pane_busy","message":"agent target pane w7:p1 is not an available shell"},"id":"cli:agent:start"}' >&2
      exit 1
    fi
    ;;
esac
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("REMUDA_HERDR_CALLS", logPath)
	t.Setenv("REMUDA_HERDR_BUSY_MARKER", busyMarker)

	starter, ok := session.NewHerdr().(session.AgentStarter)
	require.True(t, ok)
	require.NoError(t, starter.StartAgent(session.AgentStart{
		SessionName: "yendo/remuda/rm-ypr2",
		Workspace:   "/workspaces/rm-ypr2",
		Agent:       "codex",
	}))

	var starts int
	for _, call := range readHerdrCalls(t, logPath) {
		if strings.HasPrefix(call, "agent start ") {
			starts++
		}
	}
	require.Equal(t, 2, starts)
}

func TestHerdrStartAgentSubmitsMultilinePromptAfterLaunch(t *testing.T) {
	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompt")
	writeHerdrStub(t, dir, `
case "$1 $2" in
  "workspace list")
    printf '%s\n' '{"result":{"workspaces":[]}}'
    ;;
  "workspace create")
    printf '%s\n' '{"result":{"workspace":{"workspace_id":"w7"},"root_pane":{"pane_id":"w7:p1"}}}'
    ;;
  "agent start")
    for arg in "$@"; do
      case "$arg" in
        *'
'*) exit 1 ;;
      esac
    done
    ;;
  "agent prompt")
    printf '%s' "$4" > "$REMUDA_HERDR_PROMPT"
    ;;
esac
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("REMUDA_HERDR_PROMPT", promptPath)

	starter, ok := session.NewHerdr().(session.AgentStarter)
	require.True(t, ok)
	require.NoError(t, starter.StartAgent(session.AgentStart{
		SessionName: "yendo/remuda/rm-ypr2",
		Workspace:   "/workspaces/rm-ypr2",
		Agent:       "codex",
		Prompt:      "finish\nthe work",
	}))

	prompt, err := os.ReadFile(promptPath)
	require.NoError(t, err)
	require.Equal(t, "finish\nthe work", string(prompt))
}

func TestHerdrListFiltersByMetadata(t *testing.T) {
	dir := t.TempDir()
	writeHerdrStub(t, dir, `
case "$1 $2" in
  "workspace list")
    printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1","tokens":{"remuda":"1","org":"yendo","repo":"remuda","folder":"rm-ypr2","created_at":"2026-08-13T19:00:00Z"}},{"workspace_id":"w2","tokens":{}},{"workspace_id":"w3","tokens":{"remuda":"1","org":"","repo":"remuda","folder":"broken","created_at":"2026-08-13T19:00:00Z"}},{"workspace_id":"w4","tokens":{"remuda":"1","org":"yendo","repo":"remuda","folder":"unknown-age","created_at":"invalid"}}]}}'
    ;;
esac
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	sessions, err := session.NewHerdr().List()
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	require.Equal(t, "yendo/remuda/rm-ypr2", sessions[0].Name)
	require.Equal(t, "herdr", sessions[0].Multiplexer)
	require.Equal(t, "2026-08-13T19:00:00Z", sessions[0].CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	require.Equal(t, "yendo/remuda/unknown-age", sessions[1].Name)
	require.Equal(t, "herdr", sessions[1].Multiplexer)
	require.True(t, sessions[1].CreatedAt.IsZero())
}

func TestHerdrStartAgentRejectsDuplicateSession(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	writeHerdrStub(t, dir, `
case "$1 $2" in
  "workspace list")
    printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w7","tokens":{"remuda":"1","org":"yendo","repo":"remuda","folder":"rm-ypr2"}}]}}'
    ;;
esac
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("REMUDA_HERDR_CALLS", logPath)

	starter, ok := session.NewHerdr().(session.AgentStarter)
	require.True(t, ok)
	err := starter.StartAgent(session.AgentStart{SessionName: "yendo/remuda/rm-ypr2", Agent: "codex"})
	require.Error(t, err)
	var duplicate session.SessionAlreadyExistsError
	require.True(t, errors.As(err, &duplicate))
	require.Equal(t, "yendo/remuda/rm-ypr2", duplicate.Name)
	require.Equal(t, []string{"workspace list"}, readHerdrCalls(t, logPath))
}

func TestHerdrStartAgentCleansUpPartialFailure(t *testing.T) {
	tests := []struct {
		name      string
		failStage string
	}{
		{name: "metadata", failStage: "metadata"},
		{name: "agent", failStage: "agent"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "calls")
			writeHerdrStub(t, dir, `
case "$1 $2" in
  "workspace list")
    printf '%s\n' '{"result":{"workspaces":[]}}'
    ;;
  "workspace create")
    printf '%s\n' '{"result":{"workspace":{"workspace_id":"w7"},"root_pane":{"pane_id":"w7:p1"}}}'
    ;;
  "workspace report-metadata")
    if [ "$REMUDA_HERDR_FAIL_STAGE" = metadata ]; then exit 1; fi
    ;;
  "agent start")
    if [ "$REMUDA_HERDR_FAIL_STAGE" = agent ]; then exit 1; fi
    ;;
esac
`)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("REMUDA_HERDR_CALLS", logPath)
			t.Setenv("REMUDA_HERDR_FAIL_STAGE", tt.failStage)

			starter, ok := session.NewHerdr().(session.AgentStarter)
			require.True(t, ok)
			err := starter.StartAgent(session.AgentStart{
				SessionName: "yendo/remuda/rm-ypr2",
				Workspace:   "/workspaces/rm-ypr2",
				Agent:       "codex",
			})
			require.Error(t, err)
			calls := readHerdrCalls(t, logPath)
			require.Equal(t, "workspace close w7", calls[len(calls)-1])
		})
	}
}

func TestHerdrSessionOperations(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	writeHerdrStub(t, dir, `
case "$1 $2" in
  "workspace list")
    printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w7","tokens":{"remuda":"1","org":"yendo","repo":"remuda","folder":"rm-ypr2","created_at":"2026-08-13T19:00:00Z"}}]}}'
    ;;
  "pane list")
    printf '%s\n' '{"result":{"panes":[{"pane_id":"w7:p1"}]}}'
    ;;
  "pane read")
    printf 'recent output\n'
    ;;
esac
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("REMUDA_HERDR_CALLS", logPath)

	mgr := session.NewHerdr()
	info, err := mgr.Find("yendo/remuda/rm-ypr2")
	require.NoError(t, err)
	require.Equal(t, "yendo/remuda/rm-ypr2", info.Name)

	buffer, err := mgr.ReadBuffer("yendo/remuda/rm-ypr2", 25)
	require.NoError(t, err)
	require.Equal(t, "recent output\n", buffer)
	require.NoError(t, mgr.Send("yendo/remuda/rm-ypr2", "continue", true))
	require.NoError(t, mgr.Kill("yendo/remuda/rm-ypr2"))
	require.NoError(t, mgr.Attach("yendo/remuda/rm-ypr2"))

	calls := strings.Join(readHerdrCalls(t, logPath), "\n")
	require.Contains(t, calls, "pane list --workspace w7\npane read w7:p1 --source recent --lines 25")
	require.Contains(t, calls, "pane send-text w7:p1 continue\npane send-keys w7:p1 enter")
	require.Contains(t, calls, "workspace close w7")
	require.Contains(t, calls, "workspace focus w7")
}

func TestHerdrRejectsAgentCommand(t *testing.T) {
	starter, ok := session.NewHerdr().(session.AgentStarter)
	require.True(t, ok)
	err := starter.StartAgent(session.AgentStart{Agent: "custom"})
	require.Error(t, err)
	var unsupported session.UnsupportedAgentCommandError
	require.True(t, errors.As(err, &unsupported))
}

func TestNewMultiplexerCreatesHerdr(t *testing.T) {
	t.Parallel()
	require.Equal(t, "herdr", session.NewMultiplexer(session.MultiplexerHerdr).Name())
}

func writeHerdrStub(t *testing.T, dir, body string) {
	t.Helper()

	script := "#!/bin/sh\n" +
		"if [ -n \"$REMUDA_HERDR_CALLS\" ]; then printf '%s\\n' \"$*\" >> \"$REMUDA_HERDR_CALLS\"; fi\n" +
		body
	require.NoError(t, os.WriteFile(filepath.Join(dir, "herdr"), []byte(script), 0o755))
}

func readHerdrCalls(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}
