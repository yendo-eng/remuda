package internal

import (
	"github.com/yendo-eng/remuda/internal/prompts"
)

func (k Remuda) ListPrompts() ([]prompts.Prompt, error) {
	return prompts.ListWithEnv(k.envProvider())
}

func (k Remuda) ShowPrompt(name string) (string, error) {
	p, err := prompts.ResolveWithEnv(name, k.envProvider())
	if err != nil {
		return "", err
	}

	return p.Content, nil
}
