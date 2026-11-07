package agentlauncher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
)

func TestClaudeLauncher_Command_IncludesPromptWhenNonEmpty(t *testing.T) {
	l := Claude("", false, "")
	require.Equal(t, "claude 'line 1\nline 2'", l.Command("line 1\nline 2"))
}

func TestClaudeLauncher_Command_OmitsPromptWhenEmptyOrWhitespace(t *testing.T) {
	l := Claude("", false, "")
	for _, prompt := range []string{"", " ", "\n\t"} {
		require.Equal(t, "claude", l.Command(prompt))
	}
}

func TestClaudeLauncher_Command_DefaultModelIncludesEffort(t *testing.T) {
	l, resolved, err := ParseWithReasoning(string(AgentClaude), "", "high", false)
	require.NoError(t, err)
	require.Equal(t, "", resolved)
	cmd := l.Command("line 1\nline 2")
	require.Contains(t, cmd, "claude")
	require.Contains(t, cmd, "--effort 'high'")
	require.NotContains(t, cmd, "--model ")
	require.True(t, strings.HasSuffix(cmd, "'line 1\nline 2'"))
}

func TestClaudeLauncher_Command_WithModelYoloAndQuoteEscaping(t *testing.T) {
	l := Claude("claude-opus-4.1", true, "medium")
	cmd := l.Command("don't stop")
	require.Contains(t, cmd, "--model 'claude-opus-4.1'")
	require.Contains(t, cmd, "--dangerously-skip-permissions")
	require.Contains(t, cmd, "--effort 'medium'")
	require.True(t, strings.HasSuffix(cmd, "'don'\\''t stop'"))
}

func TestClaudeLauncher_Command_AgentDefaultOmitsModelFlag(t *testing.T) {
	l, resolved, err := ParseWithReasoning(string(AgentClaude), ModelAgentDefault, "low", false)
	require.NoError(t, err)
	require.Equal(t, ModelAgentDefault, resolved)
	cmd := l.Command("do stuff")
	require.NotContains(t, cmd, "--model")
	require.Contains(t, cmd, "--effort 'low'")
}

func TestClaudeLauncher_Command_EmptyPromptOmitsPositionalArg(t *testing.T) {
	l := Claude("claude-sonnet-4", false, "high")
	withPrompt := l.Command("write tests")
	withoutPrompt := l.Command("")
	require.Contains(t, withPrompt, "'write tests'")
	require.Contains(t, withoutPrompt, "--model 'claude-sonnet-4'")
	require.Contains(t, withoutPrompt, "--effort 'high'")
	require.NotContains(t, withoutPrompt, "'write tests'")
}

func TestClaudeLauncher_Command_EmptyReasoningOmitsEffort(t *testing.T) {
	l := Claude("claude-sonnet-4", false, "")
	cmd := l.Command("do stuff")
	require.Contains(t, cmd, "--model 'claude-sonnet-4'")
	require.NotContains(t, cmd, "--effort")
	require.True(t, strings.HasSuffix(cmd, "'do stuff'"))
}

func TestClaudeLauncher_Command_WithRemoteControl(t *testing.T) {
	l := Claude("claude-sonnet-4", false, "")
	withRemote, supported := l.WithRemoteControl("")
	require.True(t, supported)
	cmd := withRemote.Command("do stuff")
	require.Contains(t, cmd, "claude --model 'claude-sonnet-4' --remote-control -- 'do stuff'")
	require.NotContains(t, cmd, "--remote-control 'do stuff'")
}

func TestClaudeLauncher_Command_WithRemoteControlSession(t *testing.T) {
	l := Claude("", false, "")
	withRemote, supported := l.WithRemoteControl("my-session")
	require.True(t, supported)
	require.Contains(t, withRemote.Command("do stuff"), "claude --remote-control 'my-session' 'do stuff'")
}

func TestClaudeLauncher_Command_ModelIsShellQuoted(t *testing.T) {
	l := Claude("foo'bar; rm -rf /", false, "")
	cmd := l.Command("do stuff")
	require.Contains(t, cmd, "--model 'foo'\\''bar; rm -rf /'")
}

func TestClaudeLauncher_Command_ExtraArgsBeforePrompt(t *testing.T) {
	l := Claude("claude-sonnet-4", false, "")
	cmd := l.Command("do stuff", "--foo")
	require.Contains(t, cmd, "--model 'claude-sonnet-4' '--foo' 'do stuff'")
}

func TestClaudeLauncher_Arguments(t *testing.T) {
	l := Claude("opus", true, "high")
	require.Equal(t, []string{
		"--model", "opus",
		"--dangerously-skip-permissions",
		"--effort", "high",
		"--foo",
		"fix it",
	}, l.Arguments("fix it", "--foo"))
}

func TestBuildClaudeStateMountOpts_BothPathsPresent(t *testing.T) {
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	claudeJSON := filepath.Join(tmp, ".claude.json")

	require.NoError(t, os.MkdirAll(claudeDir, 0o700))
	require.NoError(t, os.WriteFile(claudeJSON, []byte(`{"token":"redacted"}`), 0o600))

	opts := buildClaudeStateMountOpts(zerolog.Nop(), tmp)
	require.Equal(
		t,
		[]string{
			"-v", claudeDir + ":/root/.claude:rw",
			"-v", claudeJSON + ":/root/.claude.json:rw",
		},
		opts,
	)
}

func TestClaudeLauncher_ContainerMounts_Permutations(t *testing.T) {
	tcs := []struct {
		name      string
		withDir   bool
		withFile  bool
		expectNil bool
	}{
		{name: "both", withDir: true, withFile: true},
		{name: "dir only", withDir: true},
		{name: "file only", withFile: true},
		{name: "missing", expectNil: true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			claudeDir := filepath.Join(tmp, ".claude")
			claudeJSON := filepath.Join(tmp, ".claude.json")

			if tc.withDir {
				require.NoError(t, os.MkdirAll(claudeDir, 0o700))
			}
			if tc.withFile {
				require.NoError(t, os.WriteFile(claudeJSON, []byte(`{"token":"redacted"}`), 0o600))
			}

			opts := claudeLauncher{}.ContainerMounts(zerolog.Nop(), env.StaticProvider{HomeDir: tmp})
			if tc.expectNil {
				require.Nil(t, opts)
				return
			}

			var want []string
			if tc.withDir {
				want = append(want, "-v", claudeDir+":/root/.claude:rw")
			}
			if tc.withFile {
				want = append(want, "-v", claudeJSON+":/root/.claude.json:rw")
			}
			require.Equal(t, want, opts)
		})
	}
}

func TestClaudeLauncher_ContainerMounts_BlankHome(t *testing.T) {
	opts := claudeLauncher{}.ContainerMounts(zerolog.Nop(), env.StaticProvider{HomeDir: "   "})
	require.Nil(t, opts)
}

func TestClaudeLauncher_ContainerMounts_HomeUnavailable(t *testing.T) {
	provider := env.StaticProvider{HomeErr: errors.New("no home")}
	opts := claudeLauncher{}.ContainerMounts(zerolog.Nop(), provider)
	require.Nil(t, opts)
}
