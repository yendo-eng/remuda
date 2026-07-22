package cli

import "github.com/spf13/cobra"

func (a *app) configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration.",
	}

	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file.",
		Args:  cobra.NoArgs,
	}
	a.simpleCmd(validate, nil, func([]string) error {
		_, _, err := loadConfigV1(*a.kctx)
		return err
	})

	cmd.AddCommand(validate)
	return cmd
}
