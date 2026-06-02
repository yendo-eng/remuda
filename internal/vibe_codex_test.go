package internal

import (
	"fmt"
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

	opts := codexDockerVolumeMountOptions(logging.NewDisabledLogger(), env.Default())
	expected := []string{
		fmt.Sprintf("-v %q:/root/.codex/prompts:ro", promptsDir),
		fmt.Sprintf("-v %q:/root/.codex/rules:ro", rulesDir),
		fmt.Sprintf("-v %q:/root/.codex/skills:ro", skillsDir),
		fmt.Sprintf("-v %q:/root/.codex/history.jsonl:rw", filepath.Join(tmpHome, ".codex", "history.jsonl")),
		fmt.Sprintf("-v %q:/root/.codex/sessions:rw", filepath.Join(tmpHome, ".codex", "sessions")),
	}
	require.Equal(t, expected, opts)
	require.FileExists(t, filepath.Join(tmpHome, ".codex", "history.jsonl"))
	require.DirExists(t, filepath.Join(tmpHome, ".codex", "sessions"))
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
