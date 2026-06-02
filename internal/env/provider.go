package env

import (
	"errors"
	"os"
	"sort"
)

// Provider supplies environment and home directory lookups.
type Provider interface {
	Getenv(key string) string
	LookupEnv(key string) (string, bool)
	UserHomeDir() (string, error)
	WorkingDir() (string, error)
}

// OSProvider resolves values from the process environment.
type OSProvider struct{}

func (OSProvider) Getenv(key string) string {
	return os.Getenv(key)
}

func (OSProvider) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func (OSProvider) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (OSProvider) WorkingDir() (string, error) {
	return os.Getwd()
}

func (OSProvider) Environ() []string {
	return os.Environ()
}

// Default returns the default provider backed by the process environment.
func Default() Provider {
	return OSProvider{}
}

// OrDefault returns Default when provider is nil.
func OrDefault(provider Provider) Provider {
	if provider == nil {
		return Default()
	}
	return provider
}

// ErrHomeDirUnavailable indicates that the home directory could not be resolved.
var ErrHomeDirUnavailable = errors.New("home directory not set")

// ErrWorkingDirUnavailable indicates that the working directory could not be resolved.
var ErrWorkingDirUnavailable = errors.New("working directory not set")

// StaticProvider is a map-backed provider useful for tests.
type StaticProvider struct {
	Values     map[string]string
	HomeDir    string
	HomeErr    error
	WorkDir    string
	WorkDirErr error
}

func (p StaticProvider) Getenv(key string) string {
	if val, ok := p.Values[key]; ok {
		return val
	}
	return ""
}

func (p StaticProvider) LookupEnv(key string) (string, bool) {
	val, ok := p.Values[key]
	return val, ok
}

func (p StaticProvider) UserHomeDir() (string, error) {
	if p.HomeErr != nil {
		return "", p.HomeErr
	}
	if p.HomeDir == "" {
		return "", ErrHomeDirUnavailable
	}
	return p.HomeDir, nil
}

func (p StaticProvider) WorkingDir() (string, error) {
	if p.WorkDirErr != nil {
		return "", p.WorkDirErr
	}
	if p.WorkDir == "" {
		return "", ErrWorkingDirUnavailable
	}
	return p.WorkDir, nil
}

func (p StaticProvider) Environ() []string {
	keys := make([]string, 0, len(p.Values))
	for k := range p.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+p.Values[k])
	}
	return out
}

// Environ returns the environment values from the provider when available.
// It falls back to the process environment for providers without Environ().
func Environ(provider Provider) []string {
	provider = OrDefault(provider)
	if environer, ok := provider.(interface{ Environ() []string }); ok {
		return environer.Environ()
	}
	return os.Environ()
}
