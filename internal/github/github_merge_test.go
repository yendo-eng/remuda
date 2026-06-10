package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/env"
)

func TestMergePRWithGhAppendsFlagsAndDisablesPrompt(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.txt")
	promptPath := filepath.Join(dir, "prompt.txt")

	ghPath := filepath.Join(dir, "gh")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf \"%s\\n\" \"$*\" > " + shellQuote(argsPath) + "\nprintf \"%s\\n\" \"${GH_PROMPT_DISABLED:-}\" > " + shellQuote(promptPath) + "\n"
	require.NoError(t, os.WriteFile(ghPath, []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	provider := env.StaticProvider{Values: map[string]string{"PATH": os.Getenv("PATH")}}
	err := mergePRWithGh(zerolog.Nop(), dir, 42, []string{"--squash", "--auto"}, provider)
	require.NoError(t, err)

	argsOut, readErr := os.ReadFile(argsPath)
	require.NoError(t, readErr)
	require.Equal(t, "pr merge 42 --squash --auto", strings.TrimSpace(string(argsOut)))

	promptOut, readErr := os.ReadFile(promptPath)
	require.NoError(t, readErr)
	require.Equal(t, "true", strings.TrimSpace(string(promptOut)))
}

func TestMergePRWithGhRejectsEmptyFlags(t *testing.T) {
	t.Parallel()

	err := mergePRWithGh(
		zerolog.Nop(),
		t.TempDir(),
		42,
		[]string{"--squash", " "},
		env.StaticProvider{Values: map[string]string{"PATH": os.Getenv("PATH")}},
	)
	require.ErrorContains(t, err, "merge flag at index 1 cannot be empty")
}

func TestMergePRWithGhSurfacesFailureOutput(t *testing.T) {
	dir := t.TempDir()
	ghPath := filepath.Join(dir, "gh")
	script := "#!/usr/bin/env bash\nset -euo pipefail\necho \"merge denied by policy\" >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(ghPath, []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := mergePRWithGh(
		zerolog.Nop(),
		dir,
		42,
		[]string{"--merge"},
		env.StaticProvider{Values: map[string]string{"PATH": os.Getenv("PATH")}},
	)
	require.ErrorContains(t, err, "gh pr merge #42")
	require.ErrorContains(t, err, "merge denied by policy")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
