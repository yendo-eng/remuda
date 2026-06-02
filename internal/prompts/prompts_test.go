package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
)

func TestListAndGet(t *testing.T) {
	t.Setenv(promptsDirEnv, t.TempDir())
	ps, err := List()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(ps), 1)

	p, ok := Get("small-commits")
	require.True(t, ok)
	require.Equal(t, "small-commits", p.Name)
	require.True(t, strings.Contains(p.Content, "git commit"))
}

func TestComposeOrderAndJoin(t *testing.T) {
	t.Setenv(promptsDirEnv, t.TempDir())
	// Single built-in + user prompt
	out, err := Compose([]string{"small-commits"}, "User body")
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(out, "\n\nUser body"))

	// Unknown name should error
	_, err = Compose([]string{"nope"}, "X")
	require.Error(t, err)
}

func TestGetMakePR(t *testing.T) {
	p, ok := Get("make-pr")
	require.True(t, ok)
	require.Equal(t, "make-pr", p.Name)
	require.Contains(t, p.Content, "gh pr create")
}

func TestGetUpdateDocs(t *testing.T) {
	p, ok := Get("update-docs")
	require.True(t, ok)
	require.Equal(t, "update-docs", p.Name)
	require.Contains(t, p.Content, "Make sure documentation keeps pace")
}

func TestGetRefactorCohesion(t *testing.T) {
	p, ok := Get("refactor-cohesion")
	require.True(t, ok)
	require.Equal(t, "refactor-cohesion", p.Name)
	require.Contains(t, p.Content, "Refactor to reinforce cohesion")
}

func TestGetMinimalChange(t *testing.T) {
	p, ok := Get("minimal-change")
	require.True(t, ok)
	require.Equal(t, "minimal-change", p.Name)
	require.Contains(t, p.Content, "Keep scope tight and touch only what the request requires")
}

func TestGetPrototype(t *testing.T) {
	t.Setenv(promptsDirEnv, t.TempDir())
	p, ok := Get("prototype")
	require.True(t, ok)
	require.Equal(t, "prototype", p.Name)
	require.Contains(t, p.Content, "happy path")
}

func TestResolveCustomPrompt(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv(promptsDirEnv, customDir)
	content := "please end your work by telling a joke"
	require.NoError(t, os.WriteFile(filepath.Join(customDir, "tell-jokes"), []byte(content), 0o644))

	p, err := Resolve("tell-jokes")
	require.NoError(t, err)
	require.Equal(t, "tell-jokes", p.Name)
	require.Equal(t, content, p.Content)
	require.Equal(t, "please end your work by telling a joke", p.Description)

	require.NoError(t, os.WriteFile(filepath.Join(customDir, "multiline"), []byte("first line\nsecond line"), 0o644))
	p, err = Resolve("multiline")
	require.NoError(t, err)
	require.Equal(t, "first line", p.Description)

	_, err = Resolve("../escape")
	require.Error(t, err)
	require.IsType(t, ErrInvalidPromptName(""), err)
}

func TestResolveCustomOverridesBuiltin(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv(promptsDirEnv, customDir)
	content := "custom small commits"
	require.NoError(t, os.WriteFile(filepath.Join(customDir, "small-commits"), []byte(content), 0o644))

	builtin, ok := Get("small-commits")
	require.True(t, ok)
	require.NotEqual(t, content, builtin.Content)

	p, err := Resolve("small-commits")
	require.NoError(t, err)
	require.False(t, p.Builtin)
	require.Equal(t, content, p.Content)
}

func TestListIncludesCustomPrompt(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv(promptsDirEnv, customDir)
	require.NoError(t, os.WriteFile(filepath.Join(customDir, "tell-jokes"), []byte("tell a joke"), 0o644))

	ps, err := List()
	require.NoError(t, err)
	found := false
	for _, prompt := range ps {
		if prompt.Name == "tell-jokes" {
			found = true
			break
		}
	}
	require.True(t, found, "expected custom prompt in list")
}

func TestListWithEnv_UsesProviderOverride(t *testing.T) {
	customDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(customDir, "tell-jokes"), []byte("tell a joke"), 0o644))

	provider := env.StaticProvider{
		Values: map[string]string{
			promptsDirEnv: customDir,
		},
		HomeDir: t.TempDir(),
	}

	ps, err := ListWithEnv(provider)
	require.NoError(t, err)

	found := false
	for _, prompt := range ps {
		if prompt.Name == "tell-jokes" {
			found = true
			break
		}
	}
	require.True(t, found, "expected custom prompt in list")
}
