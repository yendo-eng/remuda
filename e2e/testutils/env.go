package testutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// E2EEnvIsolationContract defines the baseline environment isolation policy for
// end-to-end tests.
//
// The contract is intentionally explicit and small. Callers are expected to:
// - Clear REMUDA_* so host configuration does not affect test behavior.
// - Force HOME/XDG dirs to a temp directory so config discovery is deterministic.
// - Force locale/TZ so output and timestamps are stable.
// - Disable git global/system config reads so user config cannot influence tests.
//
// See e2e/testutils/ENV_ISOLATION.md for the rationale and intended usage.
type E2EEnvIsolationContract struct {
	ClearPrefixes []string
	ClearExact    []string

	// AllowExact and AllowPrefixes are only applied when constructing a sanitized
	// subprocess environment.
	AllowExact    []string
	AllowPrefixes []string

	// ForcedVars are applied after any clearing/allowlisting.
	ForcedVars map[string]string
}

func DefaultE2EEnvIsolationContract(tempHome string) E2EEnvIsolationContract {
	locale := PreferredE2ELocale()
	allowExact := []string{
		// Minimal allowlist for subprocess execution. Any additional variables
		// should be opted in explicitly by tests/harness helpers.
		"PATH",
		"TERM",
		"TMPDIR",
		"TMP",
		"TEMP",
	}
	if runtime.GOOS == "windows" {
		// Common Windows environment variables needed for many programs to start.
		allowExact = append(allowExact, "SystemRoot", "ComSpec", "WINDIR", "PATHEXT")
	}

	return E2EEnvIsolationContract{
		ClearPrefixes: []string{
			"REMUDA_",

			// Git env-based config injection. We hard-disable these rather than
			// attempting to reason about caller intent.
			"GIT_CONFIG_",
			"GIT_ATTR_",
		},
		AllowExact: allowExact,
		ForcedVars: map[string]string{
			// HOME/XDG: ensure config discovery is isolated from the host.
			"HOME":            tempHome,
			"XDG_CONFIG_HOME": filepath.Join(tempHome, ".config"),
			"XDG_CACHE_HOME":  filepath.Join(tempHome, ".cache"),
			"XDG_STATE_HOME":  filepath.Join(tempHome, ".local", "state"),
			"XDG_DATA_HOME":   filepath.Join(tempHome, ".local", "share"),

			// Locale/TZ: keep output stable.
			"TZ":     "UTC",
			"LANG":   locale,
			"LC_ALL": locale,

			// Git config isolation: avoid reading user/system config files.
			"GIT_CONFIG_NOSYSTEM": "1",
			"GIT_CONFIG_GLOBAL":   GitConfigGlobalPath(tempHome),
			"GIT_CONFIG_SYSTEM":   GitConfigSystemPath(tempHome),
			"GIT_ATTR_NOSYSTEM":   "1",
		},
	}
}

var (
	preferredLocaleOnce  sync.Once
	preferredLocaleValue string
)

func PreferredE2ELocale() string {
	preferredLocaleOnce.Do(func() {
		preferredLocaleValue = detectPreferredE2ELocale()
	})
	return preferredLocaleValue
}

func detectPreferredE2ELocale() string {
	candidates := []string{"C.UTF-8", "C.utf8"}
	available, ok := localesAvailable()
	if !ok {
		return "C"
	}

	for _, c := range candidates {
		if _, exists := available[strings.ToLower(c)]; exists {
			return c
		}
	}
	return "C"
}

func localesAvailable() (map[string]struct{}, bool) {
	cmd := exec.CommandContext(context.Background(), "locale", "-a")
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}

	set := map[string]struct{}{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		set[strings.ToLower(line)] = struct{}{}
	}
	return set, true
}

func GitConfigGlobalPath(tempHome string) string {
	return filepath.Join(tempHome, ".remuda-e2e", "gitconfig-global")
}

func GitConfigSystemPath(tempHome string) string {
	return filepath.Join(tempHome, ".remuda-e2e", "gitconfig-system")
}

// SanitizeProcessEnv returns a deterministic process env slice. Unlike
// SanitizeSubprocessEnv it does not allowlist, it only clears/forces per the
// contract and then applies overrides. This is intended for suite-level
// sanitization (eg in TestMain).
func (c E2EEnvIsolationContract) SanitizeProcessEnv(parent []string, overrides map[string]string) []string {
	envMap := parseEnv(parent)
	applyClearAndForce(envMap, c.ClearExact, c.ClearPrefixes, c.ForcedVars)
	applyOverrides(envMap, overrides)
	return formatEnv(envMap)
}

func (c E2EEnvIsolationContract) EnsureFilesystem() error {
	dirs := []string{
		c.ForcedVars["HOME"],
		c.ForcedVars["XDG_CONFIG_HOME"],
		c.ForcedVars["XDG_CACHE_HOME"],
		c.ForcedVars["XDG_DATA_HOME"],
		c.ForcedVars["XDG_STATE_HOME"],
	}

	for _, p := range []string{c.ForcedVars["GIT_CONFIG_GLOBAL"], c.ForcedVars["GIT_CONFIG_SYSTEM"]} {
		if p == "" || p == os.DevNull {
			continue
		}
		dirs = append(dirs, filepath.Dir(p))
	}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	for _, p := range []string{c.ForcedVars["GIT_CONFIG_GLOBAL"], c.ForcedVars["GIT_CONFIG_SYSTEM"]} {
		if p == "" || p == os.DevNull {
			continue
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			return err
		}
	}

	return nil
}

// SanitizeSubprocessEnv returns a deterministic env slice suitable for use as
// exec.Cmd.Env. It is derived from parent but limited to the allowlist, then
// cleared/forced per the contract, and finally overridden by overrides.
func (c E2EEnvIsolationContract) SanitizeSubprocessEnv(parent []string, overrides map[string]string) []string {
	parentMap := parseEnv(parent)

	// Apply allowlist to inherited env, so host config cannot leak via unrelated
	// environment variables.
	allowed := make(map[string]string, len(parentMap))
	for k, v := range parentMap {
		if contains(c.AllowExact, k) || hasAnyPrefix(k, c.AllowPrefixes) {
			allowed[k] = v
		}
	}

	applyClearAndForce(allowed, c.ClearExact, c.ClearPrefixes, c.ForcedVars)
	applyOverrides(allowed, overrides)
	return formatEnv(allowed)
}

func parseEnv(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		out[key] = val
	}
	return out
}

func formatEnv(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	return out
}

func applyClearAndForce(dst map[string]string, clearExact, clearPrefixes []string, forced map[string]string) {
	for _, k := range clearExact {
		delete(dst, k)
	}
	for k := range dst {
		if hasAnyPrefix(k, clearPrefixes) {
			delete(dst, k)
		}
	}
	for k, v := range forced {
		dst[k] = v
	}
}

// ProcessEnvMap returns the current process environment as a map.
func ProcessEnvMap() map[string]string {
	return parseEnv(os.Environ())
}

// ApplyE2EEnvIsolationToCmd sets cmd.Env to a deterministic, allowlisted
// environment derived from baseEnv, per the default contract.
//
// Prefer this for any subprocess spawned from e2e tests so host env/config does
// not influence test behavior.
func ApplyE2EEnvIsolationToCmd(cmd *exec.Cmd, baseEnv map[string]string, overrides map[string]string) error {
	if cmd == nil {
		return nil
	}

	if baseEnv == nil {
		baseEnv = map[string]string{}
	}
	tempHome := baseEnv["HOME"]
	if tempHome == "" {
		return fmt.Errorf("ApplyE2EEnvIsolationToCmd requires HOME to be set in the base env map")
	}

	contract := DefaultE2EEnvIsolationContract(tempHome)
	if err := contract.EnsureFilesystem(); err != nil {
		return err
	}
	cmd.Env = contract.SanitizeSubprocessEnv(formatEnv(baseEnv), overrides)
	return nil
}

func applyOverrides(dst map[string]string, overrides map[string]string) {
	for k, v := range overrides {
		dst[k] = v
	}
}

func contains(xs []string, needle string) bool {
	for _, x := range xs {
		if x == needle {
			return true
		}
	}
	return false
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
