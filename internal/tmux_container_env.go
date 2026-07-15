package internal

import (
	"strings"

	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/util"
)

var tmuxSessionEnvAllowlist = []string{
	"ANTHROPIC_API_KEY",
	"BD_ACTOR",
	"BEADS_DIR",
	"DOCKER_CONFIG",
	"DOCKER_CONTEXT",
	"DOCKER_HOST",
	"EDITOR",
	"GH_TOKEN",
	"GITHUB_TOKEN",
	"GIT_HTTPS_USERNAME",
	"GIT_SSH_COMMAND",
	"GIT_TERMINAL_PROMPT",
	"HOME",
	"IS_SANDBOX",
	"LANG",
	"LC_ALL",
	"OPENAI_API_KEY",
	"PATH",
	"REMUDA_AGENT",
	"REMUDA_MODEL",
	"SHELL",
	"SSH_AUTH_SOCK",
	"TMPDIR",
	"VISUAL",
}

func tmuxSessionEnvValues(provider env.Provider, agent string, extraEnvNames, overrideEnvNames []string) []string {
	allowed := make(map[string]struct{}, len(tmuxSessionEnvAllowlist)+len(extraEnvNames)+len(overrideEnvNames)+1)
	for _, name := range tmuxSessionEnvAllowlist {
		allowed[name] = struct{}{}
	}
	for _, name := range tmuxContainerEnvNames(agent, extraEnvNames) {
		allowed[name] = struct{}{}
	}
	for _, name := range overrideEnvNames {
		name = strings.TrimSpace(name)
		if util.IsValidEnvVarName(name) {
			allowed[name] = struct{}{}
		}
	}

	var forwarded []string
	for _, kv := range launchEnvValues(provider) {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || !util.IsValidEnvVarName(name) {
			continue
		}
		if _, ok := allowed[name]; !ok && !strings.HasPrefix(name, "REMUDA_") {
			continue
		}
		forwarded = append(forwarded, kv)
	}
	return forwarded
}

// tmuxContainerEnvNames returns env var names that should be explicitly exported
// before running containerized commands in detached tmux sessions.
//
// This includes user-requested --container-inherit-env values plus implicit env
// forwards added by launch composition (currently ANTHROPIC_API_KEY for
// claude/bash) so stale tmux server environments don't drop those values.
func tmuxContainerEnvNames(agent string, containerInheritEnv []string) []string {
	names := make([]string, 0, len(containerInheritEnv)+1)
	seen := make(map[string]struct{}, len(containerInheritEnv)+1)
	add := func(raw string) {
		name := strings.TrimSpace(raw)
		if name == "" || !util.IsValidEnvVarName(name) {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	for _, name := range containerInheritEnv {
		add(name)
	}

	if strings.EqualFold(agent, "claude") || strings.EqualFold(agent, "bash") {
		add("ANTHROPIC_API_KEY")
	}

	return names
}
