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
	Skills struct {
		Directory string `yaml:"directory"`
	} `yaml:"skills"`
	Database struct {
		Driver string `yaml:"driver"`
		Path   string `yaml:"path"`
		DSN    string `yaml:"dsn"`
	} `yaml:"database"`
	CodeReview struct {
		Provider  string `yaml:"provider"`
		Endpoint  string `yaml:"endpoint"`
		Model     string `yaml:"model"`
		APIKeyEnv string `yaml:"api_key_env"`
	} `yaml:"code_review"`
}

func Default() Config {
	var c Config
	c.Server.Address = ":8081"
	c.Skills.Directory = "./data/skills"
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
	if strings.TrimSpace(c.Skills.Directory) == "" {
		return errors.New("skills.directory is required")
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
	configuredReviewFields := 0
	for _, value := range []string{c.CodeReview.Provider, c.CodeReview.Endpoint, c.CodeReview.Model} {
		if strings.TrimSpace(value) != "" {
			configuredReviewFields++
		}
	}
	if configuredReviewFields != 0 && configuredReviewFields != 3 {
		return errors.New("code_review.provider, code_review.endpoint, and code_review.model must be configured together")
	}
	return nil
}
