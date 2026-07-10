package cli

import "github.com/spf13/cobra"

func (a *app) sessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage running sessions (tmux or zellij).",
	}
	cmd.AddCommand(
		a.sessionListCmd(),
		a.sessionAttachCmd(),
		a.sessionReadbufCmd(),
		a.sessionSendCmd(),
		a.sessionPathCmd(),
		a.sessionKillCmd(),
		a.sessionInactiveCmd(),
		a.sessionResumeCmd(),
		a.sessionReapCmd(),
		a.sessionShellCmd(),
		a.sessionEditCmd(),
	)
	return cmd
}

// simpleCmd wires a command with no repo/profile resolution: assign
// positionals (if any), run the standard prepare pipeline, then the body.
func (a *app) simpleCmd(cmd *cobra.Command, fl *flagSet, body func(args []string) error) *cobra.Command {
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := a.prepare(cmd, prepareOpts{fl: fl}); err != nil {
			return err
		}
		return body(args)
	}
	return cmd
}
