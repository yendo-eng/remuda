package agentlauncher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
	shellutil "github.com/yendo-eng/remuda/internal/util/shell"
)

func TestParse_DefaultCodexModelUsesAstra(t *testing.T) {
	t.Parallel()

	launcher, model, err := Parse(string(AgentCodex), "", false)
	require.NoError(t, err)
	require.Equal(t, "gpt-6-astra", model)
	require.Equal(t, []string{"--model", "gpt-6-astra", "--", "prompt"}, launcher.Arguments("prompt"))
}

func TestCodexLauncher_Command_DashPrefixedPromptUsesArgumentTerminator(t *testing.T) {
	l := Codex("", false, "")

	for _, prompt := range []string{"-", "--name"} {
		t.Run(prompt, func(t *testing.T) {
			cmd := l.Command(prompt)
			require.Contains(t, cmd, " -- '"+prompt+"'")
		})
	}
}

func TestCodexLauncher_Command_MultiLineDashedHeaderPromptUsesArgumentTerminator(t *testing.T) {
	l := Codex("", false, "")
	prompt := "---------- Ticket RBL-123 ----------\n\nSummarize the current branch."
	cmd := l.Command(prompt)

	require.Contains(t, cmd, " -- '"+prompt+"'")
}

func TestCodexLauncher_WithRemoteControl_IsUnsupported(t *testing.T) {
	l := Codex("", false, "")
	got, supported := l.WithRemoteControl("session-1")
	require.False(t, supported)
	require.Equal(t, l.Command("prompt"), got.Command("prompt"))
}

func TestCodexLauncher_Command_ExtraArgsBeforePrompt(t *testing.T) {
	l := Codex("gpt-5.5", false, "")
	cmd := l.Command("hello", "--foo", "--bar")
	require.Contains(t, cmd, "--model 'gpt-5.5' '--foo' '--bar' -- 'hello'")
}

func TestCodexLauncher_Command_ExtraArgsAreShellQuoted(t *testing.T) {
	l := Codex("gpt-5.5", false, "")
	extra := []string{
		"--config=a,b",
		"--label=hello world",
		"--danger=$(echo hi);true",
	}
	cmd := l.Command("hello", extra...)

	for _, arg := range extra {
		require.Contains(t, cmd, shellutil.SingleQuote(arg))
	}
}

func TestCodexLauncher_Arguments(t *testing.T) {
	l := Codex("gpt-5.5", true, "high")
	require.Equal(t, []string{
		"--dangerously-bypass-approvals-and-sandbox",
		"--dangerously-bypass-hook-trust",
		"--config", "shell_environment_policy.ignore_default_excludes=true",
		"--model", "gpt-5.5",
		"--config", "model_reasoning_effort=high",
		"--foo",
		"--", "fix it",
	}, l.Arguments("fix it", "--foo"))
}

func TestCodexLauncher_ContainerMounts_IncludesPromptsMountWithAuthArtifacts(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("TMPDIR", t.TempDir())
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	promptsDir := filepath.Join(tmpHome, ".codex", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))
	rulesDir := filepath.Join(tmpHome, ".codex", "rules")
	require.NoError(t, os.MkdirAll(rulesDir, 0o755))
	skillsDir := filepath.Join(tmpHome, ".codex", "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))
	agentsPath := filepath.Join(tmpHome, ".codex", "AGENTS.md")
	require.NoError(t, os.WriteFile(agentsPath, []byte("agent instructions"), 0o644))
	cfgPath := filepath.Join(tmpHome, ".codex", "config.toml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("version = 1"), 0o644))

	opts := codexLauncher{}.ContainerMounts(zerolog.Nop(), env.Default())
	require.Contains(t, opts, "--tmpfs")
	require.Contains(t, opts, "/root/.codex:rw,mode=0755")
	require.True(t, containsMountSuffix(opts, ":/root/.codex/auth.json:ro"))
	require.True(t, containsMountSuffix(opts, ":/root/.codex/config.toml:ro"))
	require.True(t, containsMountWithSource(opts, filepath.Join(tmpHome, ".codex", "history.jsonl"), ":/root/.codex/history.jsonl:rw"))
	require.True(t, containsMountWithSource(opts, filepath.Join(tmpHome, ".codex", "sessions"), ":/root/.codex/sessions:rw"))
	require.True(t, containsMountWithSource(opts, promptsDir, ":/root/.codex/prompts:ro"))
	require.True(t, containsMountWithSource(opts, rulesDir, ":/root/.codex/rules:ro"))
	require.True(t, containsMountWithSource(opts, skillsDir, ":/root/.codex/skills:ro"))
	require.True(t, containsMountWithSource(opts, agentsPath, ":/root/.codex/AGENTS.md:ro"))
	require.FileExists(t, filepath.Join(tmpHome, ".codex", "history.jsonl"))
	require.DirExists(t, filepath.Join(tmpHome, ".codex", "sessions"))
}

func TestCodexLauncher_ContainerMounts_ForwardsPromptsWithoutOpenAIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	promptsDir := filepath.Join(tmpHome, ".codex", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))
	rulesDir := filepath.Join(tmpHome, ".codex", "rules")
	require.NoError(t, os.MkdirAll(rulesDir, 0o755))
	skillsDir := filepath.Join(tmpHome, ".codex", "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))
	agentsPath := filepath.Join(tmpHome, ".codex", "AGENTS.md")
	require.NoError(t, os.WriteFile(agentsPath, []byte("agent instructions"), 0o644))

	opts := codexLauncher{}.ContainerMounts(zerolog.Nop(), env.Default())
	expected := []string{
		"-v", promptsDir + ":/root/.codex/prompts:ro",
		"-v", rulesDir + ":/root/.codex/rules:ro",
		"-v", skillsDir + ":/root/.codex/skills:ro",
		"-v", agentsPath + ":/root/.codex/AGENTS.md:ro",
		"-v", filepath.Join(tmpHome, ".codex", "history.jsonl") + ":/root/.codex/history.jsonl:rw",
		"-v", filepath.Join(tmpHome, ".codex", "sessions") + ":/root/.codex/sessions:rw",
	}
	require.Equal(t, expected, opts)
	require.FileExists(t, filepath.Join(tmpHome, ".codex", "history.jsonl"))
	require.DirExists(t, filepath.Join(tmpHome, ".codex", "sessions"))
}

func TestCodexLauncher_ContainerMounts_ForwardsAccountAuthDirWithoutOpenAIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	codexDir := filepath.Join(tmpHome, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(`{"tokens":{}}`), 0o600))
	promptsDir := filepath.Join(codexDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	opts := codexLauncher{}.ContainerMounts(zerolog.Nop(), env.Default())
	require.Equal(t, []string{"-v", codexDir + ":/root/.codex:rw"}, opts)
}

func TestCodexLauncher_ContainerMounts_OmitsAccountAuthDirWithOpenAIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("TMPDIR", t.TempDir())
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	codexDir := filepath.Join(tmpHome, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(`{"tokens":{}}`), 0o600))

	opts := codexLauncher{}.ContainerMounts(zerolog.Nop(), env.Default())
	require.False(t, containsMountWithSource(opts, codexDir, ":/root/.codex:rw"))
	require.True(t, containsMountSuffix(opts, ":/root/.codex/auth.json:ro"))
}

func TestCodexLauncher_ContainerMounts_OmitsAccountAuthDirWhenFileMissing(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	codexDir := filepath.Join(tmpHome, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o755))

	opts := codexLauncher{}.ContainerMounts(zerolog.Nop(), env.Default())
	require.False(t, containsMountWithSource(opts, codexDir, ":/root/.codex:rw"))
}

func TestCodexLauncher_ContainerMounts_SkipsAgentsMountWhenFileMissing(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	promptsDir := filepath.Join(tmpHome, ".codex", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	opts := codexLauncher{}.ContainerMounts(zerolog.Nop(), env.Default())
	require.False(t, containsMountSuffix(opts, ":/root/.codex/AGENTS.md:ro"))
}

func containsMountSuffix(opts []string, suffix string) bool {
	for _, opt := range opts {
		if strings.HasSuffix(opt, suffix) {
			return true
		}
	}
	return false
}

func containsMountWithSource(opts []string, src string, suffix string) bool {
	for _, opt := range opts {
		if strings.Contains(opt, src) && strings.HasSuffix(opt, suffix) {
			return true
		}
	}
	return false
}
