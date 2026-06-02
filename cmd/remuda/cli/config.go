package cli

// ConfigCmd groups configuration-related subcommands.
type ConfigCmd struct {
	Validate ConfigValidateCmd `cmd:"" help:"Validate configuration file."`
}
