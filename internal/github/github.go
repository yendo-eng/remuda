package github

import (
	"encoding/json"

	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/logging"
	"github.com/yendo-eng/remuda/internal/util"
)

type GitHub interface {
	// ClosePullRequest closes the GitHub pull request associated with the given session name.
	// It returns the URL of the closed pull request or an error if the operation fails.
	ClosePullRequest(sessionName string, comment string) (*PRCloseResult, error)
	MergePullRequest(sessionName string, mergeFlags []string) (*PRMergeResult, error)

	CheckAuthStatus() error

	PRView(cwd, ref string) (map[string]any, error)
	PRViewWithRepo(repoSlug, ref string) (map[string]any, error)
	PRDiff(cwd, ref string) (string, error)
	PRCheckout(cwd, ref string) error
	IssueView(repoSlug, ref string) (*Issue, error)
}

// EnvProviderSetter allows wiring a per-invocation environment provider.
type EnvProviderSetter interface {
	WithEnv(provider env.Provider) GitHub
}

// LoggerSetter allows wiring a per-invocation logger.
type LoggerSetter interface {
	SetLogger(logger zerolog.Logger)
}

type ghCLI struct {
	env    env.Provider
	logger zerolog.Logger
}

func NewGhCLI() GitHub {
	return NewGhCLIWithEnvAndLogger(env.Default(), logging.DefaultLogger())
}

func NewGhCLIWithEnv(provider env.Provider) GitHub {
	return NewGhCLIWithEnvAndLogger(provider, logging.DefaultLogger())
}

func NewGhCLIWithEnvAndLogger(provider env.Provider, logger zerolog.Logger) GitHub {
	return &ghCLI{env: env.OrDefault(provider), logger: logger}
}

func (gh *ghCLI) WithEnv(provider env.Provider) GitHub {
	return &ghCLI{env: env.OrDefault(provider), logger: gh.logger}
}

func (gh *ghCLI) SetLogger(logger zerolog.Logger) {
	gh.logger = logger
}

func (gh *ghCLI) ClosePullRequest(workspacePath string, comment string) (*PRCloseResult, error) {
	info, err := os.Stat(workspacePath)
	if err != nil {
		if pkgerrors.Is(err, os.ErrNotExist) {
			return nil, pkgerrors.Errorf("workspace %q not found; cannot close PR", workspacePath)
		}
		return nil, pkgerrors.Wrapf(err, "stat workspace %q", workspacePath)
	}
	if !info.IsDir() {
		return nil, pkgerrors.Errorf("workspace %q is not a directory", workspacePath)
	}

	prInfo, err := fetchPRInfoFromGh(gh.logger, workspacePath, gh.env)
	if err != nil {
		if pkgerrors.Is(err, errNoPRFound) {
			return nil, nil
		}
		return nil, err
	}

	res := &PRCloseResult{
		Number: prInfo.Number,
		URL:    prInfo.URL,
		State:  prInfo.State,
	}

	if !strings.EqualFold(prInfo.State, "OPEN") {
		return res, nil
	}

	if err := closePRWithGh(gh.logger, workspacePath, prInfo.Number, comment, gh.env); err != nil {
		return nil, err
	}

	res.Closed = true
	res.State = "CLOSED"
	return res, nil
}

func (gh *ghCLI) MergePullRequest(workspacePath string, mergeFlags []string) (*PRMergeResult, error) {
	info, err := os.Stat(workspacePath)
	if err != nil {
		if pkgerrors.Is(err, os.ErrNotExist) {
			return nil, pkgerrors.Errorf("workspace %q not found; cannot merge PR", workspacePath)
		}
		return nil, pkgerrors.Wrapf(err, "stat workspace %q", workspacePath)
	}
	if !info.IsDir() {
		return nil, pkgerrors.Errorf("workspace %q is not a directory", workspacePath)
	}

	prInfo, err := fetchPRInfoFromGh(gh.logger, workspacePath, gh.env)
	if err != nil {
		if pkgerrors.Is(err, errNoPRFound) {
			return nil, nil
		}
		return nil, err
	}

	if !strings.EqualFold(prInfo.State, "OPEN") {
		return &PRMergeResult{
			Number: prInfo.Number,
			URL:    prInfo.URL,
			State:  prInfo.State,
			Merged: prInfo.MergedAt != nil,
		}, nil
	}

	if len(mergeFlags) == 0 {
		mergeFlags = []string{"--rebase"}
	}
	if err := mergePRWithGh(gh.logger, workspacePath, prInfo.Number, mergeFlags, gh.env); err != nil {
		return nil, err
	}

	return &PRMergeResult{
		Number: prInfo.Number,
		URL:    prInfo.URL,
		State:  "MERGED",
		Merged: true,
	}, nil
}

func (gh *ghCLI) CheckAuthStatus() error {
	cmd := util.CmdWithLogger(gh.logger, "gh", "auth", "status")
	cmd.Env = env.Environ(gh.env)
	err := cmd.Run()
	return pkgerrors.Wrap(err, "gh auth status")
}

func (gh *ghCLI) PRView(cwd, ref string) (map[string]any, error) {
	return ghPRView(gh.logger, cwd, "", ref, gh.env)
}

func (gh *ghCLI) PRViewWithRepo(repoSlug, ref string) (map[string]any, error) {
	return ghPRView(gh.logger, "", repoSlug, ref, gh.env)
}

func (gh *ghCLI) PRDiff(cwd, ref string) (string, error) {
	cmd := util.CmdWithLogger(gh.logger, "gh", "pr", "diff", ref)
	cmd.Dir = cwd
	cmd.Env = env.Environ(gh.env)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if pkgerrors.As(err, &ee) {
			return "", pkgerrors.Wrapf(err, "gh pr diff (stderr: %s)", string(ee.Stderr))
		}
		return "", pkgerrors.Wrap(err, "gh pr diff")
	}

	return string(out), nil
}

func (gh *ghCLI) PRCheckout(cwd, ref string) error {
	cmd := util.CmdWithLogger(gh.logger, "gh", "pr", "checkout", ref)
	cmd.Dir = cwd
	cmd.Env = env.Environ(gh.env)
	err := cmd.Run()
	return pkgerrors.Wrap(err, "gh pr checkout")
}

func ghPRView(logger zerolog.Logger, cwd, repoSlug, ref string, provider env.Provider) (map[string]any, error) {
	args := []string{"gh", "pr", "view", ref, "--json", "title,number,body,author,baseRefName,headRefName,url,labels"}
	if strings.TrimSpace(repoSlug) != "" && !strings.Contains(ref, "://") {
		args = append(args, "--repo", repoSlug)
	}
	cmd := util.CmdWithLogger(logger, args[0], args[1:]...)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	cmd.Env = env.Environ(provider)
	out, err := cmd.Output()
	if err != nil {
		// If the command writes to stderr, include it in the error context.
		var ee *exec.ExitError
		if pkgerrors.As(err, &ee) {
			return nil, pkgerrors.Wrapf(err, "gh pr view (stderr: %s)", string(ee.Stderr))
		}
		return nil, pkgerrors.Wrap(err, "gh pr view")
	}
	var info map[string]any
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, pkgerrors.Wrap(err, "parsing gh pr view output")
	}
	return info, nil
}

func (gh *ghCLI) IssueView(repoSlug, ref string) (*Issue, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, pkgerrors.Errorf("issue reference is required")
	}
	args := []string{"gh", "issue", "view", ref, "--json", "number,title,body,url,state,author,assignees,labels"}
	if repoSlug != "" && !strings.Contains(ref, "://") {
		args = append(args, "--repo", repoSlug)
	}
	cmd := util.CmdWithLogger(gh.logger, args[0], args[1:]...)
	cmd.Env = env.Environ(gh.env)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if pkgerrors.As(err, &ee) {
			return nil, pkgerrors.Wrapf(err, "gh issue view (stderr: %s)", string(ee.Stderr))
		}
		return nil, pkgerrors.Wrap(err, "gh issue view")
	}
	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, pkgerrors.Wrap(err, "parsing gh issue view output")
	}
	return &issue, nil
}

type PRCloseResult struct {
	Number int
	URL    string
	State  string
	Closed bool
}

type PRMergeResult struct {
	Number int
	URL    string
	State  string
	Merged bool
}

type ghPRInfo struct {
	Number   int        `json:"number"`
	State    string     `json:"state"`
	URL      string     `json:"url"`
	MergedAt *time.Time `json:"mergedAt"`
}

var errNoPRFound = pkgerrors.New("no pull request found")

func fetchPRInfoFromGh(logger zerolog.Logger, workspace string, provider env.Provider) (*ghPRInfo, error) {
	cmd := util.CmdWithLogger(logger, "gh", "pr", "view", "--json", "number,state,url,mergedAt")
	cmd.Dir = workspace
	cmd.Env = env.Environ(provider)
	out, err := cmd.CombinedOutput()
	if err != nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "no pull requests") || strings.Contains(lower, "pull request not found") || strings.Contains(lower, "no open pull requests") {
			return nil, errNoPRFound
		}
		return nil, pkgerrors.Wrapf(err, "gh pr view (output: %s)", strings.TrimSpace(string(out)))
	}

	var info ghPRInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, pkgerrors.Wrap(err, "parsing gh pr view output")
	}
	if info.Number == 0 {
		return nil, errNoPRFound
	}
	return &info, nil
}

func closePRWithGh(logger zerolog.Logger, workspace string, number int, comment string, provider env.Provider) error {
	args := []string{"pr", "close", strconv.Itoa(number)}
	if strings.TrimSpace(comment) != "" {
		args = append(args, "--comment", comment)
	}
	cmd := util.CmdWithLogger(logger, "gh", args...)
	cmd.Dir = workspace
	cmd.Env = append(env.Environ(provider), "GH_PROMPT_DISABLED=true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return pkgerrors.Wrapf(err, "gh pr close #%d (output: %s)", number, strings.TrimSpace(string(out)))
	}
	return nil
}

func mergePRWithGh(logger zerolog.Logger, workspace string, number int, mergeFlags []string, provider env.Provider) error {
	args := []string{"pr", "merge", strconv.Itoa(number)}

	for i, mergeFlag := range mergeFlags {
		if strings.TrimSpace(mergeFlag) == "" {
			return pkgerrors.Errorf("merge flag at index %d cannot be empty", i)
		}
		args = append(args, mergeFlag)
	}

	cmd := util.CmdWithLogger(logger, "gh", args...)
	cmd.Dir = workspace
	cmd.Env = append(env.Environ(provider), "GH_PROMPT_DISABLED=true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return pkgerrors.Wrapf(err, "gh pr merge #%d (output: %s)", number, strings.TrimSpace(string(out)))
	}
	return nil
}
