package cli

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/configfile"
)

// WorkspacesListCmd prints one workspace path per line.
type WorkspacesListCmd struct {
	Active   bool `name:"active" help:"Restrict output to workspaces with an active Remuda session."`
	Inactive bool `name:"inactive" help:"Restrict output to workspaces with no active Remuda session."`
}

func (c WorkspacesListCmd) Validate() error {
	if c.Active && c.Inactive {
		return errors.New("flags --active and --inactive cannot be used together")
	}
	return nil
}

func (c WorkspacesListCmd) Run(ctx Context) error {
	ignore := configuredWorkspacesIgnorePatterns(ctx.ConfigFile)

	var (
		workspaces []string
		err        error
	)
	if c.Active {
		workspaces, err = ctx.Remuda.ActiveWorkspacesWithIgnore(ignore)
	} else if c.Inactive {
		workspaces, err = ctx.Remuda.InactiveWorkspacesWithIgnore(ignore)
	} else {
		workspaces, err = ctx.Remuda.WorkspacesWithIgnore(ignore)
	}
	if err != nil {
		return errors.Wrap(err, "workspaces list")
	}

	for _, ws := range workspaces {
		abs, absErr := filepath.Abs(ws)
		if absErr == nil {
			ws = abs
		}
		ctx.Remuda.IO.Outln(ws)
	}

	return nil
}

func configuredWorkspacesIgnorePatterns(cfg *configfile.V1) []string {
	if cfg == nil || cfg.Workspaces == nil || cfg.Workspaces.Ignore == nil {
		return nil
	}
	values := *cfg.Workspaces.Ignore
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func configuredPruneIgnorePatterns(cfg *configfile.V1) []string {
	if cfg == nil || cfg.Session == nil || cfg.Session.Prune == nil || cfg.Session.Prune.Ignore == nil {
		return nil
	}
	values := *cfg.Session.Prune.Ignore
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}
