package cli

import (
	"strings"

	"github.com/pkg/errors"
)

// SessionEditCmd opens the workspace for a session in the configured editor.
type SessionEditCmd struct {
	SessionNamePickOption `embed:""`
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

	return "", errors.New("no editor configured; set $REMUDA_EDITOR, $VISUAL, or $EDITOR")
}
