package cli

import "github.com/spf13/cobra"

func (a *app) repoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Inspect configured repository aliases.",
	}
	cmd.AddCommand(a.repoListCmd())
	return cmd
}
