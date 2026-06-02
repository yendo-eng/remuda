package internal

// SessionSend sends input to the running session's active pane.
func (k Remuda) SessionSend(name string, payload string, appendNewline bool) error {
	return k.Session.Send(name, payload, appendNewline)
}
