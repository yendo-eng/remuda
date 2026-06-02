package cli

// WorkspacesCmd groups workspace inventory subcommands.
type WorkspacesCmd struct {
	List   WorkspacesListCmd   `cmd:"" help:"List Remuda-managed workspaces on disk."`
	Remove WorkspacesRemoveCmd `cmd:"" help:"Remove explicitly targeted workspaces."`
}
