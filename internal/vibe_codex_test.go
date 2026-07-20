package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/logging"
)

func TestCodexDockerVolumeMountOptions_IncludesPromptsMountWithAuthArtifacts(t *testing.T) {
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

	opts := codexDockerVolumeMountOptions(logging.NewDisabledLogger(), env.Default())
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

func TestCodexDockerVolumeMountOptions_ForwardsPromptsWithoutOpenAIKey(t *testing.T) {
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

	opts := codexDockerVolumeMountOptions(logging.NewDisabledLogger(), env.Default())
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

func TestCodexDockerVolumeMountOptions_ForwardsAccountAuthDirWithoutOpenAIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	codexDir := filepath.Join(tmpHome, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(`{"tokens":{}}`), 0o600))
	promptsDir := filepath.Join(codexDir, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	opts := codexDockerVolumeMountOptions(logging.NewDisabledLogger(), env.Default())
	require.Equal(t, []string{"-v", codexDir + ":/root/.codex:rw"}, opts)
}

func TestCodexDockerVolumeMountOptions_OmitsAccountAuthDirWithOpenAIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("TMPDIR", t.TempDir())
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	codexDir := filepath.Join(tmpHome, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(`{"tokens":{}}`), 0o600))

	opts := codexDockerVolumeMountOptions(logging.NewDisabledLogger(), env.Default())
	require.False(t, containsMountWithSource(opts, codexDir, ":/root/.codex:rw"))
	require.True(t, containsMountSuffix(opts, ":/root/.codex/auth.json:ro"))
}

func TestCodexDockerVolumeMountOptions_OmitsAccountAuthDirWhenFileMissing(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	codexDir := filepath.Join(tmpHome, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0o755))

	opts := codexDockerVolumeMountOptions(logging.NewDisabledLogger(), env.Default())
	require.False(t, containsMountWithSource(opts, codexDir, ":/root/.codex:rw"))
}

func TestCodexDockerVolumeMountOptions_SkipsAgentsMountWhenFileMissing(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	promptsDir := filepath.Join(tmpHome, ".codex", "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0o755))

	opts := codexDockerVolumeMountOptions(logging.NewDisabledLogger(), env.Default())
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
