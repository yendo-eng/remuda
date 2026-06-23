package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/session"
)

type fakeResumeSessionManager struct {
	name     string
	sessions []session.SessionInfo

	started  map[string]string
	startEnv map[string][]string
	attached []string
	killed   []string
}

func (f *fakeResumeSessionManager) Name() string { return f.name }

func (f *fakeResumeSessionManager) Start(sessionName, command string) error {
	if f.started == nil {
		f.started = map[string]string{}
	}
	f.started[sessionName] = command
	return nil
}

func (f *fakeResumeSessionManager) StartWithEnv(sessionName, command string, envValues []string) error {
	if err := f.Start(sessionName, command); err != nil {
		return err
	}
	if f.startEnv == nil {
		f.startEnv = map[string][]string{}
	}
	f.startEnv[sessionName] = append([]string(nil), envValues...)
	return nil
}

func (f *fakeResumeSessionManager) List() ([]session.SessionInfo, error) { return f.sessions, nil }

func (f *fakeResumeSessionManager) Find(name string) (session.SessionInfo, error) {
	for _, s := range f.sessions {
		if s.Name == name {
			return s, nil
		}
	}
	return session.SessionInfo{}, session.ErrSessionNotFound
}

func (f *fakeResumeSessionManager) Attach(name string) error {
	f.attached = append(f.attached, name)
	return nil
}

func (f *fakeResumeSessionManager) ReadBuffer(name string, lines int) (string, error) { return "", nil }

func (f *fakeResumeSessionManager) Send(name string, payload string, appendNewline bool) error {
	return nil
}

func (f *fakeResumeSessionManager) Kill(name string) error {
	f.killed = append(f.killed, name)
	return nil
}

func TestSessionResume_StartsCodexDetachedSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Detached:  true,
	})
	require.NoError(t, err)

	cmd, ok := mgr.started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "codex resume --last")
	require.NotContains(t, cmd, "REMUDA_AGENT=")
	require.NotContains(t, cmd, "export BD_ACTOR=")
	require.NotContains(t, cmd, "export BEADS_DIR=")
	require.Contains(t, cmd, "sleep 3600")
	value, ok := envValue(mgr.startEnv["org/repo/folder"], "REMUDA_AGENT")
	require.True(t, ok)
	require.Equal(t, "codex", value)
	value, ok = envValue(mgr.startEnv["org/repo/folder"], "BD_ACTOR")
	require.True(t, ok)
	require.Equal(t, "org/repo/folder", value)
}

func TestSessionResume_ClaudeStartsDetachedSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Agent:     "claude",
		Detached:  true,
	})
	require.NoError(t, err)

	cmd, ok := mgr.started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "claude --continue")
	require.NotContains(t, cmd, "codex resume --last")
	require.NotContains(t, cmd, "REMUDA_AGENT=")
	value, ok := envValue(mgr.startEnv["org/repo/folder"], "REMUDA_AGENT")
	require.True(t, ok)
	require.Equal(t, "claude", value)
}

func TestSessionResume_ClaudeYoloAndReasoningFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace:      workspace,
		Agent:          "claude",
		Detached:       true,
		Yolo:           true,
		ReasoningLevel: "high",
	})
	require.NoError(t, err)

	cmd, ok := mgr.started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "claude --continue")
	require.Contains(t, cmd, "--dangerously-skip-permissions")
	require.Contains(t, cmd, "--effort 'high'")
}

func TestSessionResume_ClaudeModelAndPromptFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Agent:     "claude",
		Model:     "claude-sonnet-4.6",
		Prompt:    "continue with tests",
		Detached:  true,
	})
	require.NoError(t, err)

	cmd, ok := mgr.started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "claude --continue")
	require.Contains(t, cmd, "--model 'claude-sonnet-4.6'")
	require.Contains(t, cmd, "'continue with tests'")
	require.NotContains(t, cmd, "REMUDA_MODEL")

	value, ok := envValue(mgr.startEnv["org/repo/folder"], "REMUDA_MODEL")
	require.True(t, ok)
	require.Equal(t, "claude-sonnet-4.6", value)
}

func TestSessionResume_DetachedTmuxExportsImplicitAnthropicForClaudeContainer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GH_TOKEN", "test-token") // avoid invoking `gh auth token`
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-secret")

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace:     workspace,
		Agent:         "claude",
		Detached:      true,
		Container:     true,
		ContainerName: "ghcr.io/acme/vibe-dev:latest",
	})
	require.NoError(t, err)

	cmd, ok := mgr.started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "-e ANTHROPIC_API_KEY")
	require.NotContains(t, cmd, "export ANTHROPIC_API_KEY=")
	value, ok := envValue(mgr.startEnv["org/repo/folder"], "ANTHROPIC_API_KEY")
	require.True(t, ok)
	require.Equal(t, "anthropic-secret", value)
}

func TestSessionResume_ContainerRequiresExplicitImage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		Docker:  &docker.Mock{Running: true},
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Detached:  true,
		Container: true,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "container mode requires an explicit image")
	require.Empty(t, mgr.started)
}

func TestSessionResume_UnsupportedAgentErrors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Agent:     "opencode",
		Detached:  true,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, `session resume unsupported for agent "opencode"`)
	require.Empty(t, mgr.started)
}

func TestSessionResume_RefusesActiveWorkspace(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	mgr := &fakeResumeSessionManager{
		name:     "tmux",
		sessions: []session.SessionInfo{{Name: "org/repo/folder"}},
	}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Detached:  true,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "active")
	require.ErrorContains(t, err, "org/repo/folder")
	require.Empty(t, mgr.started)
}

func TestSessionResume_ValidatesWorkspaceEligibility(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Detached:  true,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "depth 3")
	require.Empty(t, mgr.started)
}

func TestSessionResume_YoloAddsBypassFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Detached:  true,
		Yolo:      true,
	})
	require.NoError(t, err)

	cmd, ok := mgr.started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "codex resume --last")
	require.Contains(t, cmd, "--dangerously-bypass-approvals-and-sandbox")
	require.Contains(t, cmd, "shell_environment_policy.ignore_default_excludes")
}

func TestSessionResume_ReasoningLevelAddsConfigFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace:      workspace,
		Detached:       true,
		ReasoningLevel: "high",
	})
	require.NoError(t, err)

	cmd, ok := mgr.started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "model_reasoning_effort='high'")
}

func TestSessionResume_CustomAgentCommandAppendsPrompt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace: workspace,
		Agent:     "opencode",
		AgentCmd:  "my-resume",
		Prompt:    "pick this up",
		Detached:  true,
	})
	require.NoError(t, err)

	cmd, ok := mgr.started["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, cmd, "my-resume 'pick this up'")
	require.NotContains(t, cmd, "REMUDA_AGENT")

	value, ok := envValue(mgr.startEnv["org/repo/folder"], "REMUDA_AGENT")
	require.True(t, ok)
	require.Equal(t, "opencode", value)
}

func TestSessionResume_OpenAIKeyOverridesEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "from-env")

	base := t.TempDir()
	workspace := filepath.Join(base, "org", "repo", "folder")
	require.NoError(t, os.MkdirAll(workspace, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".beads"), 0o755))

	mgr := &fakeResumeSessionManager{name: "tmux"}
	k := Remuda{
		Config:  Config{ReposBaseDir: base},
		Session: mgr,
		IO:      DefaultIO(),
	}

	err := k.SessionResume(context.Background(), SessionResumeCommand{
		Workspace:    workspace,
		Detached:     true,
		OpenAIAPIKey: "override-key",
	})
	require.NoError(t, err)

	cmd, ok := mgr.started["org/repo/folder"]
	require.True(t, ok)
	require.NotContains(t, cmd, "OPENAI_API_KEY")

	startEnv, ok := mgr.startEnv["org/repo/folder"]
	require.True(t, ok)
	require.Contains(t, startEnv, "OPENAI_API_KEY=override-key")
}
