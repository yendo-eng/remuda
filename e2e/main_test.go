package e2e_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yendo-eng/remuda/e2e/testutils"
)

func TestMain(m *testing.M) {
	// E2E tests run a lot of git operations, which can be slow.
	flag.Parse()
	if testing.Short() {
		os.Exit(0)
	}

	// need that defer to run before os.Exit
	run := func() int {
		restore, err := sanitizeSuiteEnv()
		if err != nil {
			panic(err)
		}
		defer restore()

		return m.Run()
	}

	os.Exit(run())
}

func sanitizeSuiteEnv() (func(), error) {
	type maybeString struct {
		value string
		ok    bool
	}

	original := map[string]maybeString{}
	tempRoot := ""
	tempRootCreated := false

	cleanup := func() {
		for key, value := range original {
			if !value.ok {
				_ = os.Unsetenv(key)
				continue
			}
			_ = os.Setenv(key, value.value)
		}
		if tempRootCreated {
			_ = os.RemoveAll(tempRoot)
		}
	}

	save := func(key string) {
		if _, ok := original[key]; ok {
			return
		}
		value, ok := os.LookupEnv(key)
		original[key] = maybeString{value: value, ok: ok}
	}

	set := func(key, value string) error {
		save(key)
		return os.Setenv(key, value)
	}

	unset := func(key string) error {
		save(key)
		return os.Unsetenv(key)
	}

	var err error
	tempRoot, err = os.MkdirTemp("", "remuda-e2e-*")
	if err != nil {
		cleanup()
		return nil, err
	}
	tempRootCreated = true

	homeDir := filepath.Join(tempRoot, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		cleanup()
		return nil, err
	}

	contract := testutils.DefaultE2EEnvIsolationContract(homeDir)

	parseEnv := func(env []string) map[string]string {
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

	current := os.Environ()
	desired := contract.SanitizeProcessEnv(current, nil)
	currentMap := parseEnv(current)
	desiredMap := parseEnv(desired)

	keys := map[string]struct{}{}
	for k := range currentMap {
		keys[k] = struct{}{}
	}
	for k := range desiredMap {
		keys[k] = struct{}{}
	}

	for key := range keys {
		if val, ok := desiredMap[key]; ok {
			if err := set(key, val); err != nil {
				cleanup()
				return nil, err
			}
			continue
		}
		if err := unset(key); err != nil {
			cleanup()
			return nil, err
		}
	}

	if err := contract.EnsureFilesystem(); err != nil {
		cleanup()
		return nil, err
	}

	return cleanup, nil
}
