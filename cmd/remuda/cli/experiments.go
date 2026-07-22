package cli

import (
	"strings"

	expregistry "github.com/yendo-eng/remuda/internal/experiments"
)

func experimentCompletionValues() []string {
	experiments := expregistry.Registry()
	out := make([]string, 0, len(experiments))
	for _, exp := range experiments {
		out = append(out, exp.Name+"\t"+exp.Description)
	}
	return out
}

func validateExperiments(raw string, source string) ([]string, error) {
	return expregistry.Validate(splitFlexibleList(raw), source)
}

func experimentInputSource(rs *flagResolution, env EnvProvider) string {
	if rs != nil && rs.flagExplicit("experiments") {
		return "--experiments"
	}
	if val := strings.TrimSpace(envOrDefault(env).Getenv("REMUDA_EXPERIMENTS")); val != "" {
		return "REMUDA_EXPERIMENTS"
	}
	return "defaults.experiments"
}
