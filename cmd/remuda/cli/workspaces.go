package cli

import "github.com/spf13/cobra"

func (a *app) workspacesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspaces",
		Short: "Inspect Remuda-managed workspaces on disk.",
	}
	cmd.AddCommand(a.workspacesListCmd(), a.workspacesRemoveCmd())
	return cmd
}
