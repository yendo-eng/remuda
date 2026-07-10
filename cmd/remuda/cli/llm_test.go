package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal"
)

func TestLLMSlugify_LocalFallback(t *testing.T) {
	t.Parallel()

	// Capture output
	var out, errOut bytes.Buffer
	env := cli.EnvMap{}
	k := internal.NewRemuda(internal.Config{}, nil, nil, nil, nil, nil)
	kctx := cli.NewContext(t.Context(), k, cli.Stdout(&out), cli.Stderr(&errOut), cli.WithEnv(env), cli.WithHomeDir(t.TempDir()))

	require.NoError(t, cli.Run(kctx, []string{"llm", "slugify", "Fix: Allow --repo utils in vibe start"}))

	got := out.String()
	require.Contains(t, got, "fix-allow-repo-utils-in-vibe-start")
}
