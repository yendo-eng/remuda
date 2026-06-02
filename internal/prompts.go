package internal

import (
	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/prompts"
)

func ListPrompts() ([]prompts.Prompt, error) {
	return prompts.ListWithEnv(env.Default())
}

func ShowPrompt(name string) (string, error) {
	p, err := prompts.ResolveWithEnv(name, env.Default())
	if err != nil {
		return "", err
	}

	return p.Content, nil
}

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
