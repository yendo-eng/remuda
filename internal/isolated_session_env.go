package internal

import (
	"slices"
	"strings"

	"github.com/yendo-eng/remuda/internal/env"
	"github.com/yendo-eng/remuda/internal/util"
)

var isolatedSessionEnvAllowlist = []string{
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

// isolatedSessionEnvValues is the environment for sessions whose panes start
// from a cleared environment and load it from an env file, so only allowlisted
// values reach the agent.
func isolatedSessionEnvValues(provider env.Provider, extraEnvNames, overrideEnvNames []string) []string {
	allowed := make(map[string]struct{}, len(isolatedSessionEnvAllowlist)+len(extraEnvNames)+len(overrideEnvNames))
	for _, name := range isolatedSessionEnvAllowlist {
		allowed[name] = struct{}{}
	}
	for _, name := range slices.Concat(extraEnvNames, overrideEnvNames) {
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
