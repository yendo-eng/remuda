package cli

import "os"

func envFromContext(ctx Context) EnvProvider {
	if ctx.Env != nil {
		return ctx.Env
	}
	return defaultEnvProvider()
}

func envOrDefault(env EnvProvider) EnvProvider {
	if env != nil {
		return env
	}
	return defaultEnvProvider()
}

func environFromEnvProvider(env EnvProvider) []string {
	if env == nil {
		return os.Environ()
	}
	if environer, ok := env.(interface{ Environ() []string }); ok {
		return environer.Environ()
	}
	return os.Environ()
}

func homeDirFromContext(ctx Context) (string, error) {
	if ctx.homeDirSet {
		return ctx.HomeDir, ctx.homeDirErr
	}
	if ctx.HomeDir != "" {
		return ctx.HomeDir, ctx.homeDirErr
	}
	return defaultHomeDir()
}

func workingDirFromContext(ctx Context) string {
	if ctx.workingDirSet {
		return ctx.WorkingDir
	}
	if ctx.WorkingDir != "" {
		return ctx.WorkingDir
	}
	wd, _ := defaultWorkingDir()
	return wd
}
