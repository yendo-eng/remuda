package env

import (
	"sort"
	"strings"
	"sync"
)

// Setter allows callers to override environment values without mutating the
// process-global environment.
type Setter interface {
	Setenv(key, value string)
	Unsetenv(key string)
}

type envOverride struct {
	value string
	ok    bool
}

// MutableProvider wraps a base Provider and allows per-invocation overrides.
type MutableProvider struct {
	base      Provider
	mu        sync.RWMutex
	overrides map[string]envOverride
}

// NewMutableProvider wraps the base Provider with an override-aware provider.
// If base is already a MutableProvider, it is returned as-is.
func NewMutableProvider(base Provider) *MutableProvider {
	if mp, ok := base.(*MutableProvider); ok {
		return mp
	}
	return &MutableProvider{
		base:      OrDefault(base),
		overrides: make(map[string]envOverride),
	}
}

// Setenv sets an override value for the given key.
func (p *MutableProvider) Setenv(key, value string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.overrides[key] = envOverride{value: value, ok: true}
}

// Unsetenv masks a key from the base provider.
func (p *MutableProvider) Unsetenv(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.overrides[key] = envOverride{value: "", ok: false}
}

// Getenv returns the overridden value when present, otherwise it falls back to the base provider.
func (p *MutableProvider) Getenv(key string) string {
	val, _ := p.LookupEnv(key)
	return val
}

// LookupEnv returns the overridden value and ok when present, otherwise it falls back to the base provider.
func (p *MutableProvider) LookupEnv(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}

	p.mu.RLock()
	override, ok := p.overrides[key]
	p.mu.RUnlock()
	if ok {
		return override.value, override.ok
	}

	return p.base.LookupEnv(key)
}

func (p *MutableProvider) UserHomeDir() (string, error) {
	return p.base.UserHomeDir()
}

func (p *MutableProvider) WorkingDir() (string, error) {
	return p.base.WorkingDir()
}

// Environ returns the merged base environment with overrides applied.
func (p *MutableProvider) Environ() []string {
	baseEnv := Environ(p.base)
	envMap := make(map[string]string, len(baseEnv))
	order := make([]string, 0, len(baseEnv))
	for _, kv := range baseEnv {
		key, val := splitEnvPair(kv)
		if key == "" {
			continue
		}
		if _, ok := envMap[key]; !ok {
			order = append(order, key)
		}
		envMap[key] = val
	}

	p.mu.RLock()
	overrides := make(map[string]envOverride, len(p.overrides))
	for key, value := range p.overrides {
		overrides[key] = value
	}
	p.mu.RUnlock()

	var extraKeys []string
	for key, override := range overrides {
		if override.ok {
			if _, ok := envMap[key]; ok {
				envMap[key] = override.value
				continue
			}
			envMap[key] = override.value
			extraKeys = append(extraKeys, key)
			continue
		}
		delete(envMap, key)
	}

	sort.Strings(extraKeys)

	out := make([]string, 0, len(envMap))
	for _, key := range order {
		if val, ok := envMap[key]; ok {
			out = append(out, key+"="+val)
		}
	}
	for _, key := range extraKeys {
		if val, ok := envMap[key]; ok {
			out = append(out, key+"="+val)
		}
	}
	return out
}

func splitEnvPair(kv string) (string, string) {
	idx := strings.IndexByte(kv, '=')
	if idx < 0 {
		return kv, ""
	}
	return kv[:idx], kv[idx+1:]
}
