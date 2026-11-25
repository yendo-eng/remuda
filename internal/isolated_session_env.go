package internal

import (
	"slices"
	"strings"

	"github.com/yendo-eng/remuda/internal/util"
)

// isolatedSessionEnvNames is the session.AgentStart ForwardEnv of an agent's
// launch.
func isolatedSessionEnvNames(extraEnvNames, overrideEnvNames []string) []string {
	var names []string
	for _, name := range slices.Concat(extraEnvNames, overrideEnvNames) {
		name = strings.TrimSpace(name)
		if util.IsValidEnvVarName(name) {
			names = append(names, name)
		}
	}
	return names
}
