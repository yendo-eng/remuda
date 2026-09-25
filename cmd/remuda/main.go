package main

import (
	"context"
	"os"
	"runtime/debug"

	clipkg "github.com/yendo-eng/remuda/cmd/remuda/cli"
	"github.com/yendo-eng/remuda/internal"
	"github.com/yendo-eng/remuda/internal/logging"
)

var buildVersion string

func main() {
	cfg := internal.ConfigFromEnv()
	version := resolveVersion(buildVersion, debug.ReadBuildInfo)

	remuda := internal.NewRemuda(
		cfg,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	err := clipkg.RunWithName(
		clipkg.NewContext(context.Background(), remuda, clipkg.WithVersion(version)),
		os.Args[0],
		os.Args[1:],
	)
	if err != nil {
		logger := logging.DefaultLogger()
		logger.Error().Err(err).Msg("command failed")
		os.Exit(1)
	}
}
