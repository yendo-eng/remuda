package internal

func (k Remuda) SessionInactive() ([]string, error) {
	return k.inactiveWorkspaces(nil)
}
