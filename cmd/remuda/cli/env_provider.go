package cli

import (
	"errors"
	"os"
	"sort"
)

// EnvProvider supplies environment lookups for a single CLI invocation.
type EnvProvider interface {
	Getenv(key string) string
	LookupEnv(key string) (string, bool)
}

type osEnvProvider struct{}

func (osEnvProvider) Getenv(key string) string {
	return os.Getenv(key)
}

func (osEnvProvider) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func (osEnvProvider) Environ() []string {
	return os.Environ()
}

// EnvMap is a map-backed EnvProvider for tests.
type EnvMap map[string]string

func (m EnvMap) Getenv(key string) string {
	if val, ok := m.LookupEnv(key); ok {
		return val
	}
	return ""
}

func (m EnvMap) LookupEnv(key string) (string, bool) {
	val, ok := m[key]
	return val, ok
}

func (m EnvMap) Environ() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}

var errHomeDirUnavailable = errors.New("home directory not set")

func defaultEnvProvider() EnvProvider {
	return osEnvProvider{}
}

func defaultHomeDir() (string, error) {
	return os.UserHomeDir()
}

func defaultWorkingDir() (string, error) {
	return os.Getwd()
}
