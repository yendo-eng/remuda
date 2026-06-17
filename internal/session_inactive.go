package internal

func (k Remuda) SessionInactive() ([]string, error) {
	return k.inactiveWorkspaces(nil, false)
}

// SessionInactiveWithOptions lists inactive workspaces, optionally including
// those under the OS-temp root used by --tmp sessions.
func (k Remuda) SessionInactiveWithOptions(includeTmp bool) ([]string, error) {
	return k.inactiveWorkspaces(nil, includeTmp)
}
