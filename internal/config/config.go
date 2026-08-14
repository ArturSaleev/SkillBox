package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Address         string        `yaml:"address"`
		ReadTimeout     time.Duration `yaml:"read_timeout"`
		WriteTimeout    time.Duration `yaml:"write_timeout"`
		ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	} `yaml:"server"`
	Database struct {
		Driver  string `yaml:"driver"`
		Path    string `yaml:"path"`
		DSN     string `yaml:"dsn"`
		Migrate bool   `yaml:"migrate"`
	} `yaml:"database"`
	Auth struct {
		Mode    string   `yaml:"mode"`
		APIKeys []string `yaml:"api_keys"`
	} `yaml:"auth"`
	MCPProfiles   []MCPProfile `yaml:"mcp_profiles"`
	Observability struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"observability"`
	SeedDemo bool `yaml:"seed_demo"`
}

type MCPProfile struct {
	Slug        string   `yaml:"slug"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Permissions []string `yaml:"permissions"`
	Tools       []string `yaml:"tools"`
	Enabled     *bool    `yaml:"enabled"`
}

func Default() Config {
	var c Config
	c.Server.Address = ":8080"
	c.Server.ReadTimeout = 15 * time.Second
	c.Server.WriteTimeout = 30 * time.Second
	c.Server.ShutdownTimeout = 10 * time.Second
	c.Database.Driver = "sqlite"
	c.Database.Path = "./data/skillbox.db"
	c.Database.Migrate = true
	c.Auth.Mode = "disabled"
	c.Observability.Level = "info"
	c.Observability.Format = "json"
	return c
}
func Load(path string) (Config, error) {
	c := Default()
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return c, err
		}
		if err = yaml.Unmarshal(raw, &c); err != nil {
			return c, fmt.Errorf("decode config: %w", err)
		}
	}
	applyEnv(&c)
	return c, c.Validate()
}
func (c Config) Validate() error {
	switch c.Database.Driver {
	case "sqlite":
		if strings.TrimSpace(c.Database.Path) == "" {
			return errors.New("database.path is required for sqlite")
		}
	case "mysql", "postgres":
		if strings.TrimSpace(c.Database.DSN) == "" {
			return errors.New("database.dsn is required")
		}
	default:
		return fmt.Errorf("unsupported database driver %q", c.Database.Driver)
	}
	switch c.Auth.Mode {
	case "disabled":
	case "api_key":
		if len(c.Auth.APIKeys) == 0 {
			return errors.New("auth.api_keys is required in api_key mode")
		}
	default:
		return fmt.Errorf("unsupported auth mode %q", c.Auth.Mode)
	}
	for _, profile := range c.MCPProfiles {
		if strings.TrimSpace(profile.Slug) == "" || strings.TrimSpace(profile.Name) == "" || len(profile.Permissions) == 0 || len(profile.Tools) == 0 {
			return fmt.Errorf("mcp profile %q requires slug, name, permissions and tools", profile.Slug)
		}
	}
	return nil
}
func applyEnv(c *Config) {
	set(&c.Server.Address, "SKILLBOX_SERVER_ADDRESS")
	set(&c.Database.Driver, "SKILLBOX_DATABASE_DRIVER")
	set(&c.Database.Path, "SKILLBOX_DATABASE_PATH")
	set(&c.Database.DSN, "SKILLBOX_DATABASE_DSN")
	set(&c.Auth.Mode, "SKILLBOX_AUTH_MODE")
	set(&c.Observability.Level, "SKILLBOX_LOG_LEVEL")
	set(&c.Observability.Format, "SKILLBOX_LOG_FORMAT")
	if v := os.Getenv("SKILLBOX_API_KEYS"); v != "" {
		c.Auth.APIKeys = split(v)
	}
	boolEnv(&c.Database.Migrate, "SKILLBOX_DATABASE_MIGRATE")
	boolEnv(&c.SeedDemo, "SKILLBOX_SEED_DEMO")
}
func set(target *string, key string) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		*target = v
	}
}
func boolEnv(target *bool, key string) {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			*target = parsed
		}
	}
}
func split(v string) []string {
	var out []string
	for _, x := range strings.Split(v, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}
