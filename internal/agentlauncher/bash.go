package agentlauncher

// bashLauncher starts an interactive bash shell as the "agent".
// The prompt is ignored; this is useful for manual sessions/testing.
type bashLauncher struct{}

func Bash() AgentLauncher { return bashLauncher{} }

func (b bashLauncher) Name() string { return "bash" }

func (b bashLauncher) Command(prompt string) string { return "bash -l" }

func (b bashLauncher) WithRemoteControl(sessionName string) (AgentLauncher, bool) {
	return b, false
}

func (b bashLauncher) SupportedModels() []string { return nil }

func (b bashLauncher) Version() (string, error) { return "", nil }
