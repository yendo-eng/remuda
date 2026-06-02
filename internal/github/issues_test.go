package github

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type issueOnlyGitHub struct {
	responses map[string]*Issue
}

func (m issueOnlyGitHub) ClosePullRequest(string, string) (*PRCloseResult, error) { return nil, nil }
func (m issueOnlyGitHub) MergePullRequest(string, MergeStrategy) (*PRMergeResult, error) {
	return nil, nil
}
func (m issueOnlyGitHub) CheckAuthStatus() error                        { return nil }
func (m issueOnlyGitHub) PRView(string, string) (map[string]any, error) { return nil, nil }
func (m issueOnlyGitHub) PRViewWithRepo(string, string) (map[string]any, error) {
	return nil, nil
}
func (m issueOnlyGitHub) PRDiff(string, string) (string, error) { return "", nil }
func (m issueOnlyGitHub) PRCheckout(string, string) error       { return nil }
func (m issueOnlyGitHub) IssueView(repoSlug, ref string) (*Issue, error) {
	key := fmt.Sprintf("%s|%s", repoSlug, ref)
	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}
	return nil, fmt.Errorf("missing issue response for %s", key)
}

func TestBuildIssueContext(t *testing.T) {
	fake := issueOnlyGitHub{
		responses: map[string]*Issue{
			"acme/widgets|123": {
				Number: 123,
				Title:  "Improve caching",
				Body:   "Investigate cache misses",
				URL:    "https://github.com/acme/widgets/issues/123",
				State:  "open",
				Author: IssueActor{Login: "octocat"},
				Labels: []IssueLabel{{Name: "bug"}, {Name: "infra"}},
			},
		},
	}
	ctx, err := BuildIssueContext(fake, "acme/widgets", []string{"123"})
	require.NoError(t, err)
	require.Contains(t, ctx, "GitHub Issue acme/widgets#123")
	require.Contains(t, ctx, "Title: Improve caching")
	require.Contains(t, ctx, "State: OPEN")
	require.Contains(t, ctx, "Labels: bug, infra")
	require.Contains(t, ctx, "Body:")
	require.Contains(t, ctx, "Investigate cache misses")
	require.Contains(t, ctx, "\n\n")
}

func TestBuildIssueContextRequiresRepoForNumericRefs(t *testing.T) {
	_, err := BuildIssueContext(issueOnlyGitHub{}, "", []string{"123"})
	require.ErrorContains(t, err, "requires --repo/--repo-url")
}
