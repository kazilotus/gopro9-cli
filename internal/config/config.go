// Package config loads the optional gopro9 configuration file.
//
// Precedence everywhere in the CLI is flags > environment (GOPRO9_*) > this
// file > discovery > defaults. The file lives at
// $XDG_CONFIG_HOME/gopro9/config.yaml (~/.config/gopro9/config.yaml) or at
// the path in GOPRO9_CONFIG.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// File is the on-disk configuration.
type File struct {
	Host    string        `yaml:"host,omitempty" json:"host,omitempty"`
	Port    int           `yaml:"port,omitempty" json:"port,omitempty"`
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Output  string        `yaml:"output,omitempty" json:"output,omitempty"`
	Quiet   bool          `yaml:"quiet,omitempty" json:"quiet,omitempty"`
}

// EnvPath is the environment variable that overrides the config file location.
const EnvPath = "GOPRO9_CONFIG"

// Path returns the configuration file path for this user.
func Path() (string, error) {
	if p := os.Getenv(EnvPath); p != "" {
		return p, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("config: cannot determine config directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gopro9", "config.yaml"), nil
}

// Load reads the config file. A missing file is not an error and yields an empty File.
func Load() (File, string, error) {
	path, err := Path()
	if err != nil {
		return File{}, "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, path, nil
	}
	if err != nil {
		return File{}, path, fmt.Errorf("config: read %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return File{}, path, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return f, path, nil
}

// Save writes the config file, creating parent directories.
func Save(f File) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o600)
}
