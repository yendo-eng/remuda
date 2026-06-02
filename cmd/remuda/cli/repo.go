package cli

// RepoCmd groups repository-related subcommands.
type RepoCmd struct {
	List RepoListCmd `cmd:"" help:"List configured repository aliases."`
}
