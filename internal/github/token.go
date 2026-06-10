package github

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/util"
)

// EnsureTokenInEnvWithProvider tries to ensure GH_TOKEN/GITHUB_TOKEN are available
// via the provided environment provider. It is best-effort.
func EnsureTokenInEnvWithProvider(provider env.Provider) {
	provider = env.OrDefault(provider)
	ghToken := strings.TrimSpace(provider.Getenv("GH_TOKEN"))
	githubToken := strings.TrimSpace(provider.Getenv("GITHUB_TOKEN"))

	setter, ok := provider.(env.Setter)
	if ghToken == "" && githubToken == "" {
		if !ok {
			return
		}
		out, err := util.RunCmdOutput("gh", "auth", "token")
		if err != nil {
			return
		}
		tok := strings.TrimSpace(out)
		if tok == "" {
			return
		}
		ghToken = tok
		githubToken = tok
	}

	if ghToken == "" {
		ghToken = githubToken
	}
	if githubToken == "" {
		githubToken = ghToken
	}

	if !ok {
		return
	}
	if ghToken != "" {
		setter.Setenv("GH_TOKEN", ghToken)
	}
	if githubToken != "" {
		setter.Setenv("GITHUB_TOKEN", githubToken)
	}
}
