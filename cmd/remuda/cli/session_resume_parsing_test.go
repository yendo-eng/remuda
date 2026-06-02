package cli_test

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
)

func TestSessionResumeCmdParse_WithWorkspaceDir(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"session", "resume", "/tmp/workspace"})
	require.NoError(t, err)
}

func TestSessionResumeCmdParse_WithPick(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"session", "resume", "--pick"})
	require.NoError(t, err)
}

func TestSessionResumeCmdParse_RequiresExactlyOneMode(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"session", "resume"})
	require.Error(t, err)

	_, err = parser.Parse([]string{"session", "resume", "/tmp/workspace", "--pick"})
	require.Error(t, err)
}

func TestSessionResumeCmdParse_RejectsBlankWorkspaceDir(t *testing.T) {
	t.Parallel()
	var c cli.CLI
	parser := kong.Must(&c, kong.Name("remuda"))

	_, err := parser.Parse([]string{"session", "resume", "   "})
	require.Error(t, err)
}

func TestSessionResumeCmdParse_NegatableYoloFlag(t *testing.T) {
	t.Parallel()
	env := cli.EnvMap{"REMUDA_YOLO": "true"}
	parser, parsed, _ := newParserWithEnv(t, env)

	_, err := parser.Parse([]string{"session", "resume", "/tmp/workspace", "--no-yolo"})
	require.NoError(t, err)
	require.False(t, parsed.Session.Resume.Yolo)
}
