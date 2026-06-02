package internal

func (k Remuda) SessionReadBuffer(name string, lines int) (string, error) {
	return k.Session.ReadBuffer(name, lines)
}
