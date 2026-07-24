package experiments

import (
	"strings"

	pkgerrors "github.com/pkg/errors"
)

const UsePromptsContextWrapper = "use-prompts-context-wrapper"

// CoWClone populates --full-clone workspaces with copy-on-write clones of the
// repo cache instead of byte copies.
const CoWClone = "cow-clone"

// SessionManifest writes a .remuda.json launch manifest into the workspace on
// vibe, and has session resume read it back to default flags that weren't
// passed explicitly.
const SessionManifest = "session-manifest"

type Experiment struct {
	Name        string
	Description string
}

func Registry() []Experiment {
	return []Experiment{
		{
			Name:        UsePromptsContextWrapper,
			Description: "wrap saved prompt context before injecting it into the agent prompt",
		},
		{
			Name:        CoWClone,
			Description: "populate --full-clone workspaces with copy-on-write clones of the repo cache instead of byte copies",
		},
		{
			Name:        SessionManifest,
			Description: "write a .remuda.json launch manifest into the workspace and have session resume default flags from it",
		},
	}
}

func RetiredRegistry() map[string]string {
	return map[string]string{
		"auto-workspace-name": "was mainlined and is now a no-op; remove it",
	}
}

func RetiredReason(name string) (string, bool) {
	reason, ok := RetiredRegistry()[strings.ToLower(strings.TrimSpace(name))]
	return reason, ok
}

func Validate(names []string, source string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}

	valid := make(map[string]struct{}, len(Registry()))
	validNames := make([]string, 0, len(Registry()))
	for _, exp := range Registry() {
		name := strings.ToLower(strings.TrimSpace(exp.Name))
		valid[name] = struct{}{}
		validNames = append(validNames, exp.Name)
	}

	retired := RetiredRegistry()
	var retiredNames []string
	seenRetired := map[string]struct{}{}
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		normalized := strings.ToLower(trimmed)
		if _, ok := valid[normalized]; ok {
			continue
		}
		if _, ok := retired[normalized]; ok {
			if _, seen := seenRetired[normalized]; !seen {
				retiredNames = append(retiredNames, normalized)
				seenRetired[normalized] = struct{}{}
			}
			continue
		}
		return retiredNames, pkgerrors.Errorf("%s: unknown experiment %q (valid: %s)", source, name, strings.Join(validNames, ", "))
	}
	return retiredNames, nil
}
