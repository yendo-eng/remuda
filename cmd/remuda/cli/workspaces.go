package cli

// WorkspacesCmd groups workspace inventory subcommands.
type WorkspacesCmd struct {
	List   WorkspacesListCmd   `cmd:"" help:"List Remuda-managed workspaces on disk."`
	Rename WorkspacesRenameCmd `cmd:"" help:"Rename an inactive workspace path and default branch."`
	Remove WorkspacesRemoveCmd `cmd:"" help:"Remove explicitly targeted workspaces."`
}
