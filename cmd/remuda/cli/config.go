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
		cfg, _, err := loadConfigV1(*a.kctx)
		if err != nil {
			return err
		}
		return validateConfigExperiments(cfg, a.warnRetiredExperiment)
	})

	cmd.AddCommand(validate)
	return cmd
}
