package agentlauncher

// debugLauncher is like the bash launcher, but it runs the prompt itself as
// a command. Useful for e2e testing and debugging.
type debugLauncher struct{}

func Debug() AgentLauncher { return debugLauncher{} }

func (b debugLauncher) Name() string { return "debug" }

func (b debugLauncher) Command(prompt string, extraArgs ...string) string { return prompt }

func (b debugLauncher) WithRemoteControl(sessionName string) (AgentLauncher, bool) {
	return b, false
}

func (b debugLauncher) SupportedModels() []string { return nil }

func (b debugLauncher) Version() (string, error) { return "", nil }
