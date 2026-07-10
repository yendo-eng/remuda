package cli

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// SessionEditCmd opens the workspace for a session in the configured editor.
type SessionEditCmd struct {
	SessionNamePickOption
}

func (a *app) sessionEditCmd() *cobra.Command {
	c := &SessionEditCmd{}
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Open the workspace for a session in your configured editor.",
		Args:  cobra.NoArgs,
	}
	c.SessionNamePickOption.register(cmd)
	return a.simpleCmd(cmd, nil, func([]string) error { return c.Run(*a.kctx) })
}

func (c SessionEditCmd) Run(ctx Context) error {
	sessionName, err := c.SessionName(ctx)
	if err != nil {
		return err
	}

	editorCmd, err := ResolveEditorCommand(envFromContext(ctx))
	if err != nil {
		return err
	}

	return ctx.Remuda.SessionEdit(strings.TrimSpace(sessionName), editorCmd)
}

// ResolveEditorCommand determines which editor command to run for session edit.
func ResolveEditorCommand(env EnvProvider) (string, error) {
	env = envOrDefault(env)
	for _, key := range []string{"REMUDA_EDITOR", "VISUAL", "EDITOR"} {
		if cmd := strings.TrimSpace(env.Getenv(key)); cmd != "" {
			return cmd, nil
		}
	}

	return "", pkgerrors.New("no editor configured; set $REMUDA_EDITOR, $VISUAL, or $EDITOR")
}
