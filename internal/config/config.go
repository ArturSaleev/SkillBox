package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Address string `yaml:"address"`
	} `yaml:"server"`
	Database struct {
		Driver string `yaml:"driver"`
		Path   string `yaml:"path"`
		DSN    string `yaml:"dsn"`
	} `yaml:"database"`
}

func Default() Config {
	var c Config
	c.Server.Address = ":8081"
	c.Database.Driver = "sqlite"
	c.Database.Path = "./data/skillbox.db"
	return c
}

func Load(path string) (Config, error) {
	c := Default()
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return c, err
		}
		decoder := yaml.NewDecoder(bytes.NewReader(raw))
		decoder.KnownFields(true)
		if err = decoder.Decode(&c); err != nil {
			return c, fmt.Errorf("decode config: %w", err)
		}
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.Address) == "" {
		return errors.New("server.address is required")
	}
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
	return nil
}
