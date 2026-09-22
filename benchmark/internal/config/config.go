package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const maskedSecret = "••••••••"

var safeID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)

type Config struct {
	Version    int         `yaml:"version" json:"version"`
	Server     Server      `yaml:"server" json:"server"`
	Database   Database    `yaml:"database" json:"database"`
	Providers  []Provider  `yaml:"providers" json:"providers"`
	MCPServers []MCPServer `yaml:"mcp_servers" json:"mcp_servers"`
	Runner     Runner      `yaml:"runner" json:"runner"`
}

type Server struct {
	Address string `yaml:"address" json:"address"`
}

type Database struct {
	Path string `yaml:"path" json:"path"`
}

type Provider struct {
	ID             string `yaml:"id" json:"id"`
	Name           string `yaml:"name" json:"name"`
	Type           string `yaml:"type" json:"type"`
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	BaseURL        string `yaml:"base_url" json:"base_url"`
	APIKey         string `yaml:"api_key,omitempty" json:"-"`
	APIKeyEnv      string `yaml:"api_key_env,omitempty" json:"api_key_env,omitempty"`
	DefaultModel   string `yaml:"default_model,omitempty" json:"default_model,omitempty"`
	RequestTimeout string `yaml:"request_timeout,omitempty" json:"request_timeout,omitempty"`
}

type MCPServer struct {
	ID             string `yaml:"id" json:"id"`
	Name           string `yaml:"name" json:"name"`
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	Transport      string `yaml:"transport" json:"transport"`
	URL            string `yaml:"url" json:"url"`
	AuthHeader     string `yaml:"auth_header,omitempty" json:"auth_header,omitempty"`
	AuthValue      string `yaml:"auth_value,omitempty" json:"-"`
	AuthValueEnv   string `yaml:"auth_value_env,omitempty" json:"auth_value_env,omitempty"`
	RequestTimeout string `yaml:"request_timeout,omitempty" json:"request_timeout,omitempty"`
}

type Runner struct {
	MaxSteps     int `yaml:"max_steps" json:"max_steps"`
	ParallelRuns int `yaml:"parallel_runs" json:"parallel_runs"`
}

type PublicConfig struct {
	Version    int               `json:"version"`
	Server     Server            `json:"server"`
	Database   Database          `json:"database"`
	Providers  []PublicProvider  `json:"providers"`
	MCPServers []PublicMCPServer `json:"mcp_servers"`
	Runner     Runner            `json:"runner"`
}

type PublicProvider struct {
	Provider
	APIKey           string `json:"api_key,omitempty"`
	APIKeyConfigured bool   `json:"api_key_configured"`
}

type PublicMCPServer struct {
	MCPServer
	AuthValue           string `json:"auth_value,omitempty"`
	AuthValueConfigured bool   `json:"auth_value_configured"`
}

type Manager struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

func Default() Config {
	return Config{
		Version:  1,
		Server:   Server{Address: "127.0.0.1:8091"},
		Database: Database{Path: "./benchmark/data/skillbox-benchmark.db"},
		Runner:   Runner{MaxSteps: 20, ParallelRuns: 2},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	return cfg, cfg.Validate()
}

func NewManager(path string) (*Manager, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	manager := &Manager{path: path, cfg: cfg}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := manager.write(cfg); err != nil {
			return nil, err
		}
	}
	return manager, nil
}

func (m *Manager) Current() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return clone(m.cfg)
}

func (m *Manager) Public() PublicConfig {
	return Public(m.Current())
}

func Public(cfg Config) PublicConfig {
	result := PublicConfig{Version: cfg.Version, Server: cfg.Server, Database: cfg.Database, Runner: cfg.Runner, Providers: []PublicProvider{}, MCPServers: []PublicMCPServer{}}
	for _, item := range cfg.Providers {
		public := PublicProvider{Provider: item, APIKeyConfigured: resolveSecret(item.APIKey, item.APIKeyEnv) != ""}
		public.Provider.APIKey = ""
		if public.APIKeyConfigured {
			public.APIKey = maskedSecret
		}
		result.Providers = append(result.Providers, public)
	}
	for _, item := range cfg.MCPServers {
		public := PublicMCPServer{MCPServer: item, AuthValueConfigured: resolveSecret(item.AuthValue, item.AuthValueEnv) != ""}
		public.MCPServer.AuthValue = ""
		if public.AuthValueConfigured {
			public.AuthValue = maskedSecret
		}
		result.MCPServers = append(result.MCPServers, public)
	}
	return result
}

func FromPublic(input PublicConfig) Config {
	result := Config{Version: input.Version, Server: input.Server, Database: input.Database, Runner: input.Runner}
	for _, item := range input.Providers {
		provider := item.Provider
		provider.APIKey = item.APIKey
		result.Providers = append(result.Providers, provider)
	}
	for _, item := range input.MCPServers {
		server := item.MCPServer
		server.AuthValue = item.AuthValue
		result.MCPServers = append(result.MCPServers, server)
	}
	return result
}

// Update validates and atomically persists the complete editable configuration.
// Empty or masked secret values preserve the existing value for the same ID.
func (m *Manager) Update(next Config) (restartRequired bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mergeSecrets(&next, m.cfg)
	if err := next.Validate(); err != nil {
		return false, err
	}
	restartRequired = next.Server.Address != m.cfg.Server.Address || next.Database.Path != m.cfg.Database.Path
	if err := m.write(next); err != nil {
		return false, err
	}
	m.cfg = clone(next)
	return restartRequired, nil
}

func (m *Manager) Reload() error {
	next, err := Load(m.path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = next
	m.mu.Unlock()
	return nil
}

func (m *Manager) write(cfg Config) error {
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".skillbox-bench-config-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	encoder := yaml.NewEncoder(tmp)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		encoder.Close()
		tmp.Close()
		return err
	}
	if err := encoder.Close(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, m.path); err != nil {
		return err
	}
	return os.Chmod(m.path, 0o600)
}

func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if strings.TrimSpace(c.Server.Address) == "" {
		return errors.New("server.address is required")
	}
	if strings.TrimSpace(c.Database.Path) == "" {
		return errors.New("database.path is required")
	}
	if c.Runner.MaxSteps <= 0 || c.Runner.MaxSteps > 100 {
		return errors.New("runner.max_steps must be between 1 and 100")
	}
	if c.Runner.ParallelRuns <= 0 || c.Runner.ParallelRuns > 32 {
		return errors.New("runner.parallel_runs must be between 1 and 32")
	}
	seen := map[string]string{}
	for _, provider := range c.Providers {
		if err := validateID("provider", provider.ID, seen); err != nil {
			return err
		}
		if strings.TrimSpace(provider.Name) == "" || strings.TrimSpace(provider.BaseURL) == "" {
			return fmt.Errorf("provider %q requires name and base_url", provider.ID)
		}
		if provider.Type != "openai_compatible" {
			return fmt.Errorf("provider %q has unsupported type %q", provider.ID, provider.Type)
		}
		if _, err := timeout(provider.RequestTimeout, 120*time.Second); err != nil {
			return fmt.Errorf("provider %q: %w", provider.ID, err)
		}
	}
	seen = map[string]string{}
	for _, server := range c.MCPServers {
		if err := validateID("MCP server", server.ID, seen); err != nil {
			return err
		}
		if strings.TrimSpace(server.Name) == "" || strings.TrimSpace(server.URL) == "" {
			return fmt.Errorf("MCP server %q requires name and url", server.ID)
		}
		if server.Transport != "streamable_http" {
			return fmt.Errorf("MCP server %q has unsupported transport %q", server.ID, server.Transport)
		}
		if _, err := timeout(server.RequestTimeout, 30*time.Second); err != nil {
			return fmt.Errorf("MCP server %q: %w", server.ID, err)
		}
	}
	return nil
}

func ProviderTimeout(value string) time.Duration {
	result, _ := timeout(value, 120*time.Second)
	return result
}

func MCPTimeout(value string) time.Duration {
	result, _ := timeout(value, 30*time.Second)
	return result
}

func (p Provider) ResolvedAPIKey() string     { return resolveSecret(p.APIKey, p.APIKeyEnv) }
func (m MCPServer) ResolvedAuthValue() string { return resolveSecret(m.AuthValue, m.AuthValueEnv) }

func timeout(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	result, err := time.ParseDuration(value)
	if err != nil || result <= 0 {
		return 0, fmt.Errorf("invalid request_timeout %q", value)
	}
	return result, nil
}

func validateID(kind, id string, seen map[string]string) error {
	id = strings.TrimSpace(id)
	if !safeID.MatchString(id) {
		return fmt.Errorf("%s id %q is invalid", kind, id)
	}
	if previous := seen[id]; previous != "" {
		return fmt.Errorf("duplicate %s id %q", kind, id)
	}
	seen[id] = id
	return nil
}

func resolveSecret(inline, envName string) string {
	if strings.TrimSpace(envName) != "" {
		if value := os.Getenv(envName); value != "" {
			return value
		}
	}
	return inline
}

func mergeSecrets(next *Config, previous Config) {
	providers := make(map[string]Provider, len(previous.Providers))
	for _, item := range previous.Providers {
		providers[item.ID] = item
	}
	for i := range next.Providers {
		old := providers[next.Providers[i].ID]
		if next.Providers[i].APIKey == "" || next.Providers[i].APIKey == maskedSecret {
			next.Providers[i].APIKey = old.APIKey
		}
	}
	servers := make(map[string]MCPServer, len(previous.MCPServers))
	for _, item := range previous.MCPServers {
		servers[item.ID] = item
	}
	for i := range next.MCPServers {
		old := servers[next.MCPServers[i].ID]
		if next.MCPServers[i].AuthValue == "" || next.MCPServers[i].AuthValue == maskedSecret {
			next.MCPServers[i].AuthValue = old.AuthValue
		}
	}
}

func clone(cfg Config) Config {
	raw, _ := yaml.Marshal(cfg)
	var result Config
	_ = yaml.Unmarshal(raw, &result)
	return result
}
