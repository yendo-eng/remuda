package cli

import (
	"errors"

	"io/fs"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
)

const (
	configOverrideEnvVar = "REMUDA_CONFIG"
)

type ConfigFileSource string

const (
	ConfigFileSourceNone   ConfigFileSource = ""
	ConfigFileSourceEnv    ConfigFileSource = "env"
	ConfigFileSourceXDG    ConfigFileSource = "xdg"
	ConfigFileSourceLegacy ConfigFileSource = "legacy"
)

type ConfigFileDiscovery struct {
	Path   string
	Strict bool
	Source ConfigFileSource
}

// DiscoverConfigFile returns the config file path to use (if any).
//
// Search order:
//  1. $REMUDA_CONFIG (strict; if set, only this path is used)
//  2. $XDG_CONFIG_HOME/remuda/config.yaml
//  3. ~/.config/remuda/config.yaml (only when XDG_CONFIG_HOME is unset)
//  4. ~/.remuda/config.yaml (legacy)
//
// If no config file is found, Path is empty and error is nil.
func DiscoverConfigFile(kctx Context) (ConfigFileDiscovery, error) {
	env := envFromContext(kctx)
	home, homeErr := homeDirFromContext(kctx)
	workingDir := workingDirFromContext(kctx)

	if override := strings.TrimSpace(env.Getenv(configOverrideEnvVar)); override != "" {
		expanded, err := expandHomePath(override, home, homeErr)
		if err != nil {
			return ConfigFileDiscovery{}, err
		}
		expanded = resolvePathFromWorkingDir(expanded, workingDir)
		if err := requireReadableFile(expanded); err != nil {
			return ConfigFileDiscovery{}, err
		}
		return ConfigFileDiscovery{
			Path:   expanded,
			Strict: true,
			Source: ConfigFileSourceEnv,
		}, nil
	}

	xdgHome := strings.TrimSpace(env.Getenv("XDG_CONFIG_HOME"))

	if xdgHome != "" {
		xdgHome = resolvePathFromWorkingDir(xdgHome, workingDir)
		xdgPath := filepath.Join(xdgHome, "remuda", "config.yaml")
		if ok, err := readableFileExists(xdgPath); err != nil {
			return ConfigFileDiscovery{}, err
		} else if ok {
			return ConfigFileDiscovery{
				Path:   xdgPath,
				Strict: false,
				Source: ConfigFileSourceXDG,
			}, nil
		}
	}

	if homeErr != nil || home == "" {
		// No home means we can't look up default (~/.config) or legacy (~/.remuda) paths.
		return ConfigFileDiscovery{Source: ConfigFileSourceNone}, nil
	}

	if xdgHome == "" {
		xdgFallbackPath := filepath.Join(home, ".config", "remuda", "config.yaml")
		if ok, err := readableFileExists(xdgFallbackPath); err != nil {
			return ConfigFileDiscovery{}, err
		} else if ok {
			return ConfigFileDiscovery{
				Path:   xdgFallbackPath,
				Strict: false,
				Source: ConfigFileSourceXDG,
			}, nil
		}
	}

	legacyPath := filepath.Join(home, ".remuda", "config.yaml")
	if ok, err := readableFileExists(legacyPath); err != nil {
		return ConfigFileDiscovery{}, err
	} else if ok {
		return ConfigFileDiscovery{
			Path:   legacyPath,
			Strict: false,
			Source: ConfigFileSourceLegacy,
		}, nil
	}

	return ConfigFileDiscovery{Source: ConfigFileSourceNone}, nil
}

func readableFileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, pkgerrors.Wrapf(err, "stat config file %q", path)
	}
	if info.IsDir() {
		return false, pkgerrors.Errorf("config file path %q is a directory", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return false, pkgerrors.Wrapf(err, "open config file %q", path)
	}
	_ = f.Close()

	return true, nil
}

func requireReadableFile(path string) error {
	ok, err := readableFileExists(path)
	if err != nil {
		return err
	}
	if !ok {
		return pkgerrors.Errorf("config file %q does not exist", path)
	}
	return nil
}

func expandHomePath(path string, home string, homeErr error) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	if homeErr != nil || home == "" {
		if homeErr == nil {
			homeErr = errHomeDirUnavailable
		}
		return "", pkgerrors.Wrapf(homeErr, "expand home %q", path)
	}
	if path == "~" {
		return home, nil
	}
	sep := string(filepath.Separator)
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+sep) {
		return filepath.Join(home, path[2:]), nil
	}
	return "", pkgerrors.Errorf("expand home %q: unsupported tilde path", path)
}
