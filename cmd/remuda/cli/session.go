package cli

// SessionCmd is a grouping command for session management.
type SessionCmd struct {
	List     SessionListCmd     `cmd:"" help:"List active sessions created by Remuda."`
	Attach   SessionAttachCmd   `cmd:"" help:"Attach to an existing session."`
	Readbuf  SessionReadbufCmd  `cmd:"" help:"Print the current pane buffer for logs (tail)."`
	Send     SessionSendCmd     `cmd:"" help:"Send a prompt to a running session."`
	Path     SessionPathCmd     `cmd:"" help:"Print the absolute workspace path for a session."`
	Kill     SessionKillCmd     `cmd:"" help:"Kill one or all sessions (optionally clean up workspace)."`
	Inactive SessionInactiveCmd `cmd:"" help:"Print inactive workspace paths (no active session), one per line."`
	Resume   SessionResumeCmd   `cmd:"" help:"Resume the most recent supported agent session in an inactive workspace."`
	Reap     SessionReapCmd     `cmd:"" help:"Kill active sessions older than a threshold (safe with --dry-run)."`
	Shell    SessionShellCmd    `cmd:"" help:"Open a shell for a session (container when available; use --host to force a host shell in the workspace)."`
	Edit     SessionEditCmd     `cmd:"" help:"Open the workspace for a session in your configured editor."`
}
