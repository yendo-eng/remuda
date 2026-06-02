package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/cmd/remuda/cli"
)

func TestSessionSendParse_MultipleNames(t *testing.T) {
	t.Parallel()
	parser, parsed, _ := newParserWithEnv(t, cli.EnvMap{})

	_, err := parser.Parse([]string{
		"session",
		"send",
		"--name",
		"org/repo/one",
		"--name",
		"org/repo/two",
		"hello",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"org/repo/one", "org/repo/two"}, parsed.Session.Send.Names)
}

func TestSessionSendParse_NameAndPickConflict(t *testing.T) {
	t.Parallel()
	parser, _, _ := newParserWithEnv(t, cli.EnvMap{})

	_, err := parser.Parse([]string{
		"session",
		"send",
		"--name",
		"org/repo/one",
		"--pick",
		"hello",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "--name and --pick")
}
