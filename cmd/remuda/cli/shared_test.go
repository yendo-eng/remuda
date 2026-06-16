package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/jira"
)

func TestContextEngineeringOptionsNoUseFiltersUse(t *testing.T) {
	t.Parallel()
	env := EnvMap{"REMUDA_PROMPTS_DIR": t.TempDir()}
	ctx := newTestContextWithEnv(t, env)

	opts := ContextEngineeringOptions{
		Use:   []PromptName{"make-pr", "small-commits"},
		NoUse: []PromptName{"make-pr"},
	}

	added, err := opts.AddedPromptContext(ctx, PromptContextInput{})
	require.NoError(t, err)
	require.Len(t, added, 1)
	require.Contains(t, added[0], "git commit")
	require.NotContains(t, added[0], "gh pr create")
}

func TestContextEngineeringOptionsWrapsUsePromptsWithContextTags(t *testing.T) {
	t.Parallel()
	env := EnvMap{"REMUDA_PROMPTS_DIR": t.TempDir()}
	ctx := newTestContextWithEnv(t, env)

	opts := ContextEngineeringOptions{
		Use: []PromptName{"make-pr", "small-commits"},
	}

	added, err := opts.AddedPromptContext(ctx, PromptContextInput{WrapUsePrompts: true})
	require.NoError(t, err)
	require.Len(t, added, 1)
	require.Equal(t, 1, strings.Count(added[0], "<context>"))
	require.Equal(t, 1, strings.Count(added[0], "</context>"))
}

func TestShouldAddMainPromptMarker(t *testing.T) {
	t.Parallel()
	require.True(t, shouldAddMainPromptMarker(true, true))
	require.False(t, shouldAddMainPromptMarker(false, true))
	require.False(t, shouldAddMainPromptMarker(true, false))
	require.False(t, shouldAddMainPromptMarker(false, false))
}

func TestContextEngineeringOptionsNoUseFiltersEnvDefaults(t *testing.T) {
	t.Parallel()
	env := EnvMap{
		"REMUDA_PROMPTS_DIR": t.TempDir(),
		"REMUDA_USE_PROMPTS": "make-pr,small-commits",
	}
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("remuda"), kong.Bind(&Context{Env: env}), kong.Resolvers(NewEnvResolver(env)))
	require.NoError(t, err)

	_, err = parser.Parse([]string{"vibe", "--no-use", "make-pr", "--name", "wk", "hi"})
	require.NoError(t, err)

	ctx := newTestContextWithEnv(t, env)
	added, err := cli.Vibe.AddedPromptContext(ctx, PromptContextInput{})
	require.NoError(t, err)
	require.Len(t, added, 1)
	require.Contains(t, added[0], "git commit")
	require.NotContains(t, added[0], "gh pr create")
}

func TestContextEngineeringOptionsValidatePromptUsage_AllowsNoUseWithEmptyPrompt(t *testing.T) {
	t.Parallel()
	opts := ContextEngineeringOptions{
		Use:   []PromptName{"make-pr"},
		NoUse: []PromptName{"make-pr"},
	}

	err := opts.validatePromptUsage("", []string{"vibe", "--no-use", "make-pr"})
	require.NoError(t, err)
}

func TestContextEngineeringOptionsValidatePromptUsage_ErrorsOnExplicitUseWithEmptyPrompt(t *testing.T) {
	t.Parallel()
	opts := ContextEngineeringOptions{
		Use: []PromptName{"make-pr"},
	}

	err := opts.validatePromptUsage("", []string{"vibe", "--use", "make-pr"})
	require.ErrorContains(t, err, "--use/-u requires a non-empty prompt")
}

func TestContextEngineeringOptionsValidatePromptUsage_ErrorsOnContextFlagsWithEmptyPrompt(t *testing.T) {
	t.Parallel()
	opts := ContextEngineeringOptions{
		Jira: []string{"PROJ-1"},
	}

	err := opts.validatePromptUsage("", nil)
	require.ErrorContains(t, err, "prompt context flags")
}

func TestContextEngineeringOptionsValidatePromptNames_IgnoresUnreadableUnrelatedPrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	env := EnvMap{"REMUDA_PROMPTS_DIR": dir}
	badPath := filepath.Join(dir, "bad-prompt")
	require.NoError(t, os.WriteFile(badPath, []byte("broken"), 0o600))
	require.NoError(t, os.Chmod(badPath, 0o000))
	t.Cleanup(func() {
		_ = os.Chmod(badPath, 0o600)
	})

	opts := ContextEngineeringOptions{
		Use: []PromptName{"small-commits"},
	}
	ctx := newTestContextWithEnv(t, env)
	require.NoError(t, opts.validatePromptNames(ctx))
}

func TestVibeNoUseCommaSeparatedParses(t *testing.T) {
	t.Parallel()
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("remuda"), kong.Bind(&Context{}))
	require.NoError(t, err)

	_, err = parser.Parse([]string{"vibe", "--no-use", "make-pr,small-commits", "--name", "wk", "hi"})
	require.NoError(t, err)
	require.Equal(t, []PromptName{"make-pr", "small-commits"}, cli.Vibe.NoUse)
}

func TestVibeJiraLowercaseNormalizesToUppercaseDuringParse(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("remuda"), kong.Bind(&Context{}))
	require.NoError(t, err)

	_, err = parser.Parse([]string{"vibe", "--name", "wk", "--jira", "proj-123", "hi"})
	require.NoError(t, err)
	require.Equal(t, []string{"PROJ-123"}, cli.Vibe.Jira)
}

func TestVibeJiraInvalidKeyReturnsParseErrorWithoutContextBinding(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("remuda"), kong.Bind((*Context)(nil)))
	require.NoError(t, err)

	_, err = parser.Parse([]string{"vibe", "--name", "wk", "--jira", "not-a-key", "hi"})
	require.ErrorContains(t, err, "invalid --jira value")
	require.ErrorContains(t, err, "expected format ABC-123")
}

func TestVibeJiraMixedOrderIsPreservedAfterNormalization(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("remuda"), kong.Bind(&Context{}))
	require.NoError(t, err)

	_, err = parser.Parse([]string{
		"vibe",
		"--name", "wk",
		"--jira", "abc-1",
		"--jira", " r2d2-42 ",
		"--jira", "ZZ-7",
		"hi",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"ABC-1", "R2D2-42", "ZZ-7"}, cli.Vibe.Jira)
}

func TestVibeJiraAuthFlagsTrimWhitespaceDuringParse(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("remuda"), kong.Bind(&Context{}))
	require.NoError(t, err)

	_, err = parser.Parse([]string{
		"vibe",
		"--name", "wk",
		"--jira-endpoint", " https://jira.example.atlassian.net/ ",
		"--jira-user", " dev@example.com ",
		"--jira-token", " token-123 ",
		"hi",
	})
	require.NoError(t, err)
	require.Equal(t, "https://jira.example.atlassian.net/", cli.Vibe.JiraEndpoint)
	require.Equal(t, "dev@example.com", cli.Vibe.JiraUser)
	require.Equal(t, "token-123", cli.Vibe.JiraToken)
}

func TestContextEngineeringOptionsPassesJiraAuthOverrideToRuntime(t *testing.T) {
	t.Parallel()

	j := &jiraAuthCapture{
		ticketBody: "ticket details",
	}
	ctx := newTestContextWithEnv(t, EnvMap{}, func(c *Context) {
		c.Remuda.Jira = j
	})
	opts := ContextEngineeringOptions{
		Jira:         []string{"PROJ-101"},
		JiraEndpoint: "https://jira.example.atlassian.net",
		JiraUser:     "dev@example.com",
		JiraToken:    "secret-token",
	}

	added, err := opts.AddedPromptContext(ctx, PromptContextInput{})
	require.NoError(t, err)
	require.Len(t, added, 1)
	require.Equal(t, jira.AuthConfig{
		Endpoint: "https://jira.example.atlassian.net",
		User:     "dev@example.com",
		Token:    "secret-token",
	}, j.lastAuthConfig)
	require.Equal(t, []string{"PROJ-101"}, j.requestedTickets)
}

type jiraAuthCapture struct {
	lastAuthConfig   jira.AuthConfig
	requestedTickets []string
	ticketBody       string
}

func (j *jiraAuthCapture) SetAuthConfigOverride(cfg jira.AuthConfig) {
	j.lastAuthConfig = cfg
}

func (j *jiraAuthCapture) GetTicket(id string) (string, error) {
	j.requestedTickets = append(j.requestedTickets, id)
	return j.ticketBody, nil
}

func TestWorkspacePickerDisplayName(t *testing.T) {
	t.Parallel()
	reposBase := filepath.FromSlash("/home/dev/.remuda/repos")
	tmpBase := filepath.FromSlash("/var/folders/xy/remuda")

	t.Run("persistent workspace renders org/repo/folder", func(t *testing.T) {
		ws := filepath.Join(reposBase, "org", "repo", "wk")
		name, ok := workspacePickerDisplayName(ws, reposBase, tmpBase)
		require.True(t, ok)
		require.Equal(t, "org/repo/wk", name)
	})

	t.Run("tmp workspace renders with (tmp) suffix and no ../ noise", func(t *testing.T) {
		ws := filepath.Join(tmpBase, "org", "repo", "wk")
		name, ok := workspacePickerDisplayName(ws, reposBase, tmpBase)
		require.True(t, ok)
		require.Equal(t, "org/repo/wk (tmp)", name)
		require.NotContains(t, name, "..")
	})

	t.Run("same-named persistent and tmp worktrees get distinct labels", func(t *testing.T) {
		persistent, okP := workspacePickerDisplayName(filepath.Join(reposBase, "org", "repo", "wk"), reposBase, tmpBase)
		tmp, okT := workspacePickerDisplayName(filepath.Join(tmpBase, "org", "repo", "wk"), reposBase, tmpBase)
		require.True(t, okP)
		require.True(t, okT)
		require.NotEqual(t, persistent, tmp)
	})

	t.Run("path under neither root is skipped", func(t *testing.T) {
		_, ok := workspacePickerDisplayName(filepath.FromSlash("/somewhere/else/org/repo/wk"), reposBase, tmpBase)
		require.False(t, ok)
	})

	t.Run("empty tmp base does not panic and tmp paths are skipped", func(t *testing.T) {
		ws := filepath.Join(tmpBase, "org", "repo", "wk")
		_, ok := workspacePickerDisplayName(ws, reposBase, "")
		require.False(t, ok)
	})
}
