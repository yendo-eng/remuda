package cli

import "os"

type contextDirs struct {
	home    *fixedPath
	working *fixedPath
}

type fixedPath struct {
	value string
	err   error
}

func fixedPathFor(value string, unavailable error) *fixedPath {
	if value == "" {
		return &fixedPath{err: unavailable}
	}
	return &fixedPath{value: value}
}

func (d contextDirs) homeDir() (string, error) {
	if d.home != nil {
		return d.home.value, d.home.err
	}
	return defaultHomeDir()
}

func (d contextDirs) workingDir() (string, error) {
	if d.working != nil {
		return d.working.value, d.working.err
	}
	return defaultWorkingDir()
}

func (c Context) env() EnvProvider {
	return envOrDefault(c.Env)
}

func (c Context) homeDir() (string, error) {
	return c.dirs.homeDir()
}

func (c Context) workingDir() string {
	workingDir, _ := c.dirs.workingDir()
	return workingDir
}

func envOrDefault(env EnvProvider) EnvProvider {
	if env != nil {
		return env
	}
	return defaultEnvProvider()
}

func environFromEnvProvider(env EnvProvider) []string {
	if environer, ok := envOrDefault(env).(interface{ Environ() []string }); ok {
		return environer.Environ()
	}
	return os.Environ()
}

type contextEnvProvider struct {
	base EnvProvider
	dirs contextDirs
}

func (p contextEnvProvider) Getenv(key string) string {
	return p.base.Get(key)
}

func (p contextEnvProvider) LookupEnv(key string) (string, bool) {
	return p.base.Lookup(key)
}

func (p contextEnvProvider) UserHomeDir() (string, error) {
	return p.dirs.homeDir()
}

func (p contextEnvProvider) WorkingDir() (string, error) {
	return p.dirs.workingDir()
}

func (p contextEnvProvider) Environ() []string {
	return environFromEnvProvider(p.base)
}
