package testutils

import (
	"encoding/json"
	"fmt"

	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/util"
)

type MockGitHub struct {
	RepoURL          string
	FakePRData       string
	ClosedWorkspaces []string
	ClosedComments   []string
	MergedWorkspaces []string
	MergeErr         error
	Issues           map[string]*github.Issue
	Env              map[string]string
}

func (m *MockGitHub) ClosePullRequest(sessionName string, comment string) (*github.PRCloseResult, error) {
	m.ClosedWorkspaces = append(m.ClosedWorkspaces, sessionName)
	m.ClosedComments = append(m.ClosedComments, comment)
	return &github.PRCloseResult{
		Number: 1,
		URL:    m.RepoURL + "/pull/1",
		State:  "CLOSED",
		Closed: true,
	}, nil
}

func (m *MockGitHub) MergePullRequest(workspace string, strategy github.MergeStrategy) (*github.PRMergeResult, error) {
	m.MergedWorkspaces = append(m.MergedWorkspaces, workspace)
	if m.MergeErr != nil {
		return nil, m.MergeErr
	}
	return &github.PRMergeResult{
		Number: 1,
		URL:    m.RepoURL + "/pull/1",
		State:  "MERGED",
		Merged: true,
	}, nil
}

func (m *MockGitHub) CheckAuthStatus() error {
	return nil
}

func (m *MockGitHub) PRView(cwd, ref string) (map[string]any, error) {
	defaultJsonStr := `{
		"author": {
			"id": "MDQ6VXNlcjE2MDc0MDkx",
			"is_bot": false,
			"login": "alex",
			"name": "Alex Example"
		},
		"baseRefName": "master",
		"body": "test",
		"headRefName": "head-ref-name",
		"labels": [],
		"number": 1,
		"title": "pr title",
		"url": "` + m.RepoURL + `/pull/1"
	}`
	if m.FakePRData == "" {
		m.FakePRData = defaultJsonStr
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(m.FakePRData), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (m *MockGitHub) PRViewWithRepo(repoSlug, ref string) (map[string]any, error) {
	return m.PRView("", ref)
}

func (m *MockGitHub) PRDiff(cwd, ref string) (string, error) {
	return "diff --git a/file.txt b/file.txt\nindex e69de29..b6fc4c6 100644\n--- a/file.txt\n+++ b/file.txt\n@@ -0,0 +1 @@\n+new line\n", nil
}

func (m *MockGitHub) PRCheckout(cwd, ref string) error {
	// Just checkout the branch with git instead
	cmd := util.Cmd("git", "-C", cwd, "checkout", ref)
	baseEnv := m.Env
	if baseEnv == nil {
		baseEnv = ProcessEnvMap()
	}
	if err := ApplyE2EEnvIsolationToCmd(cmd, baseEnv, nil); err != nil {
		return err
	}
	return cmd.Run()
}

func (m *MockGitHub) IssueView(repoSlug, ref string) (*github.Issue, error) {
	if m.Issues == nil {
		return nil, fmt.Errorf("mock github has no issues configured for %s|%s", repoSlug, ref)
	}
	key := fmt.Sprintf("%s|%s", repoSlug, ref)
	if issue, ok := m.Issues[key]; ok {
		return issue, nil
	}
	return nil, fmt.Errorf("mock github missing issue for %s|%s", repoSlug, ref)
}
