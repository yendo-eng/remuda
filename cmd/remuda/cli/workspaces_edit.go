package cli

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// WorkspacesEditCmd opens an explicitly targeted workspace in the configured editor.
type WorkspacesEditCmd struct {
	Target string
}

func (a *app) workspacesEditCmd() *cobra.Command {
	c := &WorkspacesEditCmd{}
	cmd := &cobra.Command{
		Use:   "edit <target>",
		Short: "Open a workspace in your configured editor.",
		Args:  cobra.ExactArgs(1),
	}
	registerWorkspaceDirPositionalCompletion(cmd)
	return a.simpleCmd(cmd, nil, func(args []string) error {
		c.Target = args[0]
		if err := c.Validate(); err != nil {
			return err
		}
		return c.Run(*a.kctx)
	})
}

func (c WorkspacesEditCmd) Validate() error {
	return validateWorkspaceTarget(c.Target)
}

func (c WorkspacesEditCmd) Run(ctx Context) error {
	resolved, err := resolveWorkspaceTargets([]string{c.Target}, ctx)
	if err != nil {
		return err
	}

	editorCmd, err := ResolveEditorCommand(envFromContext(ctx))
	if err != nil {
		return err
	}

	return ctx.Remuda.WorkspacesEdit(resolved[0], editorCmd)
}

// ResolveEditorCommand determines which editor command to run.
func ResolveEditorCommand(env EnvProvider) (string, error) {
	env = envOrDefault(env)
	for _, key := range []string{"REMUDA_EDITOR", "VISUAL", "EDITOR"} {
		if cmd := strings.TrimSpace(env.Getenv(key)); cmd != "" {
			return cmd, nil
		}
	}

	return "", pkgerrors.New("no editor configured; set $REMUDA_EDITOR, $VISUAL, or $EDITOR")
}
