package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/jira"
)

func TestContextEngineeringOptionsNoUseFiltersUse(t *testing.T) {
	t.Parallel()
	env := EnvMap{"REMUDA_PROMPTS_DIR": t.TempDir()}
	ctx := newTestContextWithEnv(t, env)

	opts := ContextEngineeringOptions{
		Use:   []string{"make-pr", "small-commits"},
		NoUse: []string{"make-pr"},
	}

	added, err := opts.AddedPromptContext(ctx, PromptContextInput{})
	require.NoError(t, err)
	require.Len(t, added.UsePrompts, 1)
	require.Empty(t, added.Reference)
	require.Contains(t, added.UsePrompts[0], "git commit")
	require.NotContains(t, added.UsePrompts[0], "gh pr create")
}

func TestContextEngineeringOptionsWrapsUsePromptsWithContextTags(t *testing.T) {
	t.Parallel()
	env := EnvMap{"REMUDA_PROMPTS_DIR": t.TempDir()}
	ctx := newTestContextWithEnv(t, env)

	opts := ContextEngineeringOptions{
		Use: []string{"make-pr", "small-commits"},
	}

	added, err := opts.AddedPromptContext(ctx, PromptContextInput{WrapUsePrompts: true})
	require.NoError(t, err)
	require.Len(t, added.UsePrompts, 1)
	require.Equal(t, 1, strings.Count(added.UsePrompts[0], "<context>"))
	require.Equal(t, 1, strings.Count(added.UsePrompts[0], "</context>"))
}

func TestShouldAddMainPromptMarker(t *testing.T) {
	t.Parallel()
	require.True(t, shouldAddMainPromptMarker(true, true))
	require.False(t, shouldAddMainPromptMarker(false, true))
	require.False(t, shouldAddMainPromptMarker(true, false))
	require.False(t, shouldAddMainPromptMarker(false, false))
}

func TestArrangePromptContext(t *testing.T) {
	t.Parallel()

	parts := PromptContextParts{
		UsePrompts: []string{"saved"},
		Reference:  []string{"jira"},
	}
	tests := []struct {
		name       string
		position   string
		addMarker  bool
		wantBefore []string
		wantAfter  []string
	}{
		{
			name:       "before preserves saved prompt first",
			position:   "before",
			addMarker:  true,
			wantBefore: []string{"saved", "jira", "Main prompt:"},
		},
		{
			name:       "after keeps reference before main prompt",
			position:   "after",
			addMarker:  true,
			wantBefore: []string{"jira", "Main prompt:"},
			wantAfter:  []string{"saved"},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			before, after := arrangePromptContext(parts, test.position, test.addMarker)
			require.Equal(t, test.wantBefore, before)
			require.Equal(t, test.wantAfter, after)
		})
	}
}

func TestContextEngineeringOptionsValidatePromptUsage_AllowsNoUseWithEmptyPrompt(t *testing.T) {
	t.Parallel()
	opts := ContextEngineeringOptions{
		Use:   []string{"make-pr"},
		NoUse: []string{"make-pr"},
	}

	err := opts.validatePromptUsage(contextWithExplicitFlags(Context{}, "no-use"), "")
	require.NoError(t, err)
}

func TestContextEngineeringOptionsValidatePromptUsage_ErrorsOnExplicitUseWithEmptyPrompt(t *testing.T) {
	t.Parallel()
	opts := ContextEngineeringOptions{
		Use: []string{"make-pr"},
	}

	err := opts.validatePromptUsage(contextWithExplicitFlags(Context{}, "use"), "")
	require.ErrorContains(t, err, "--use/-u requires a non-empty prompt")
}

func TestContextEngineeringOptionsValidatePromptUsage_AllowsEnvDefaultsWithEmptyPrompt(t *testing.T) {
	t.Parallel()
	opts := ContextEngineeringOptions{
		Use: []string{"make-pr"},
	}

	// Use prompts sourced from env/config (not the --use flag) must not force a prompt.
	err := opts.validatePromptUsage(Context{}, "")
	require.NoError(t, err)
}

func TestContextEngineeringOptionsValidatePromptUsage_ErrorsOnContextFlagsWithEmptyPrompt(t *testing.T) {
	t.Parallel()
	opts := ContextEngineeringOptions{
		Jira: []string{"PROJ-1"},
	}

	err := opts.validatePromptUsage(Context{}, "")
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
		Use: []string{"small-commits"},
	}
	ctx := newTestContextWithEnv(t, env)
	require.NoError(t, opts.validatePromptNames(ctx))
}

func TestContextEngineeringOptionsAfterApplyNormalizesJira(t *testing.T) {
	t.Parallel()
	ctx := newTestContextWithEnv(t, EnvMap{"REMUDA_PROMPTS_DIR": t.TempDir()})

	opts := ContextEngineeringOptions{
		Jira:         []string{"abc-1", " r2d2-42 ", "ZZ-7"},
		JiraEndpoint: " https://jira.example.atlassian.net/ ",
		JiraUser:     " dev@example.com ",
		JiraToken:    " token-123 ",
	}
	require.NoError(t, opts.afterApply(ctx))
	require.Equal(t, []string{"ABC-1", "R2D2-42", "ZZ-7"}, opts.Jira)
	require.Equal(t, "https://jira.example.atlassian.net/", opts.JiraEndpoint)
	require.Equal(t, "dev@example.com", opts.JiraUser)
	require.Equal(t, "token-123", opts.JiraToken)
}

func TestContextEngineeringOptionsAfterApplyRejectsInvalidJiraKey(t *testing.T) {
	t.Parallel()
	ctx := newTestContextWithEnv(t, EnvMap{"REMUDA_PROMPTS_DIR": t.TempDir()})

	opts := ContextEngineeringOptions{Jira: []string{"not-a-key"}}
	err := opts.afterApply(ctx)
	require.ErrorContains(t, err, "invalid --jira value")
	require.ErrorContains(t, err, "expected format ABC-123")
}

func TestContextEngineeringOptionsAfterApplyMergesGhIssueAlias(t *testing.T) {
	t.Parallel()
	ctx := newTestContextWithEnv(t, EnvMap{"REMUDA_PROMPTS_DIR": t.TempDir()})

	opts := ContextEngineeringOptions{
		GitHubIssue:  []string{"https://github.com/acme/utils/issues/1"},
		ghIssueAlias: []string{"42"},
	}
	require.NoError(t, opts.afterApply(ctx))
	require.Equal(t, []string{"https://github.com/acme/utils/issues/1", "42"}, opts.GitHubIssue)
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
	require.Empty(t, added.UsePrompts)
	require.Len(t, added.Reference, 1)
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
