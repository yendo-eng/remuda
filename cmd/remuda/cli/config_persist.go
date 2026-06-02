package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/yendo-eng/remuda/internal/configfile"
)

func persistDefaultRepoSelection(kctx Context, alias string, url string) (ConfigFileDiscovery, error) {
	alias = strings.TrimSpace(alias)
	url = strings.TrimSpace(url)
	if alias == "" && url == "" {
		return ConfigFileDiscovery{}, errors.New("repo alias or URL is required")
	}
	if alias != "" && url != "" {
		return ConfigFileDiscovery{}, errors.New("repo alias and URL are mutually exclusive")
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
		return nil, discovery, nil, errors.New("no config file path available")
	}

	info, err := os.Stat(discovery.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &configfile.V1{Version: configfile.Version1}, discovery, nil, nil
		}
		return nil, discovery, nil, err
	}
	if info.IsDir() {
		return nil, discovery, nil, fmt.Errorf("config file path %q is a directory", discovery.Path)
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
		return fmt.Errorf("stat config file %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("config file path %q is a directory", path)
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
		return "", fmt.Errorf("resolve user home dir: %w", homeErr)
	}
	return filepath.Join(home, ".config", "remuda", "config.yaml"), nil
}

func writeConfigV1(path string, cfg *configfile.V1, original []byte) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("config file path is required")
	}
	if cfg == nil {
		return errors.New("config is required")
	}

	data, err := renderConfigV1(cfg, original)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config temp file: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod config temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config temp file: %w", err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace config file: %w", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read written config: %w", err)
	}
	if _, err := configfile.ParseV1(written); err != nil {
		return fmt.Errorf("validate written config: %w", err)
	}

	return nil
}
