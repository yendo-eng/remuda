package main

import (
	"context"
	"os"
	"runtime/debug"

	clipkg "github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/docker"
	"github.com/yendo-eng/remuda/internal/git"
	"github.com/yendo-eng/remuda/internal/github"
	"github.com/yendo-eng/remuda/internal/jira"
	"github.com/yendo-eng/remuda/internal/logging"
)

var buildVersion string

func main() {
	cfg := internal.ConfigFromEnv()
	version := resolveVersion(buildVersion, debug.ReadBuildInfo)

	k := internal.NewRemuda(
		cfg,
		git.NewShellGit(),
		// TODO: there may be a nicer way of doing this while still keeping it testable
		nil, // leave the session manager null for cli to set up
		jira.NewHTTPJira(),
		docker.NewShellDocker(),
		github.NewGhCLI(),
	)

	err := clipkg.RunWithName(
		clipkg.NewContext(context.Background(), k, clipkg.WithVersion(version)),
		os.Args[0],
		os.Args[1:],
	)
	if err != nil {
		logger := logging.DefaultLogger()
		logger.Error().Err(err).Msg("command failed")
		os.Exit(1)
	}
}
