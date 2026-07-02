package cli

import (
	"errors"

	"io/fs"
	"os"
	"path/filepath"
	"strings"

	pkgerrors "github.com/pkg/errors"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func persistDefaultRepoSelection(kctx Context, alias string, url string) (ConfigFileDiscovery, error) {
	alias = strings.TrimSpace(alias)
	url = strings.TrimSpace(url)
	if alias == "" && url == "" {
		return ConfigFileDiscovery{}, pkgerrors.New("repo alias or URL is required")
	}
	if alias != "" && url != "" {
		return ConfigFileDiscovery{}, pkgerrors.New("repo alias and URL are mutually exclusive")
	}

	cfg, discovery, raw, err := loadConfigForRepoPersistence(kctx)
	if err != nil {
		return discovery, err
	}

	if cfg.Repos == nil {
		cfg.Repos = &configfile.ReposV1{}
	}
	if alias != "" {
		cfg.Repos.DefaultRepo = &alias
		cfg.Repos.DefaultRepoURL = nil
	}
	if url != "" {
		cfg.Repos.DefaultRepoURL = &url
		cfg.Repos.DefaultRepo = nil
	}

	if err := writeConfigV1(discovery.Path, cfg, raw); err != nil {
		return discovery, err
	}

	return discovery, nil
}

func loadConfigForRepoPersistence(kctx Context) (*configfile.V1, ConfigFileDiscovery, []byte, error) {
	discovery, err := discoverConfigFileForWrite(kctx)
	if err != nil {
		return nil, discovery, nil, err
	}
	if discovery.Path == "" {
		return nil, discovery, nil, pkgerrors.New("no config file path available")
	}

	info, err := os.Stat(discovery.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &configfile.V1{Version: configfile.Version1}, discovery, nil, nil
		}
		return nil, discovery, nil, err
	}
	if info.IsDir() {
		return nil, discovery, nil, pkgerrors.Errorf("config file path %q is a directory", discovery.Path)
	}

	data, err := os.ReadFile(discovery.Path)
	if err != nil {
		return nil, discovery, nil, err
	}
	cfg, err := configfile.ParseV1(data)
	if err != nil {
		return nil, discovery, nil, err
	}

	return cfg, discovery, data, nil
}

func discoverConfigFileForWrite(kctx Context) (ConfigFileDiscovery, error) {
	env := envFromContext(kctx)
	home, homeErr := homeDirFromContext(kctx)
	workingDir := workingDirFromContext(kctx)

	discovery, err := DiscoverConfigFile(kctx)
	if err == nil {
		if discovery.Path != "" {
			return discovery, nil
		}

		primary, perr := primaryConfigPath(kctx)
		if perr != nil {
			return discovery, perr
		}
		discovery.Path = primary
		discovery.Source = ConfigFileSourceXDG
		return discovery, nil
	}

	override := strings.TrimSpace(env.Getenv(configOverrideEnvVar))
	if override == "" {
		return discovery, err
	}
	expanded, perr := expandHomePath(override, home, homeErr)
	if perr != nil {
		return discovery, perr
	}
	expanded = resolvePathFromWorkingDir(expanded, workingDir)
	if perr := ensureConfigPathWritable(expanded); perr != nil {
		return discovery, perr
	}

	return ConfigFileDiscovery{
		Path:   expanded,
		Strict: true,
		Source: ConfigFileSourceEnv,
	}, nil
}

func ensureConfigPathWritable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return pkgerrors.Wrapf(err, "stat config file %q", path)
	}
	if info.IsDir() {
		return pkgerrors.Errorf("config file path %q is a directory", path)
	}
	return nil
}

func primaryConfigPath(kctx Context) (string, error) {
	env := envFromContext(kctx)
	home, homeErr := homeDirFromContext(kctx)
	workingDir := workingDirFromContext(kctx)

	if xdgHome := strings.TrimSpace(env.Getenv("XDG_CONFIG_HOME")); xdgHome != "" {
		xdgHome = resolvePathFromWorkingDir(xdgHome, workingDir)
		return filepath.Join(xdgHome, "remuda", "config.yaml"), nil
	}
	if homeErr != nil || home == "" {
		if homeErr == nil {
			homeErr = errHomeDirUnavailable
		}
		return "", pkgerrors.Wrap(homeErr, "resolve user home dir")
	}
	return filepath.Join(home, ".config", "remuda", "config.yaml"), nil
}

func writeConfigV1(path string, cfg *configfile.V1, original []byte) error {
	if strings.TrimSpace(path) == "" {
		return pkgerrors.New("config file path is required")
	}
	if cfg == nil {
		return pkgerrors.New("config is required")
	}

	data, err := renderConfigV1(cfg, original)
	if err != nil {
		return pkgerrors.Wrap(err, "marshal config")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return pkgerrors.Wrapf(err, "create config directory %q", dir)
	}

	tmp, err := os.CreateTemp(dir, "config-*.yaml")
	if err != nil {
		return pkgerrors.Wrap(err, "create temp config")
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return pkgerrors.Wrap(err, "write config temp file")
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return pkgerrors.Wrap(err, "chmod config temp file")
	}
	if err := tmp.Close(); err != nil {
		return pkgerrors.Wrap(err, "close config temp file")
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return pkgerrors.Wrap(err, "replace config file")
	}

	written, err := os.ReadFile(path)
	if err != nil {
		return pkgerrors.Wrap(err, "read written config")
	}
	if _, err := configfile.ParseV1(written); err != nil {
		return pkgerrors.Wrap(err, "validate written config")
	}

	return nil
}
