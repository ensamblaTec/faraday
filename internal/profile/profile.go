// Package profile loads and saves per-app faraday sandbox profiles (YAML).
package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Profile is a saved sandbox configuration for one app.
type Profile struct {
	Description string   `yaml:"description,omitempty"`
	Flatpak     string   `yaml:"flatpak,omitempty"` // flatpak app id; non-empty → flatpak backend
	Loopback    bool     `yaml:"loopback,omitempty"`
	Cmd         []string `yaml:"cmd,omitempty"` // command+args for non-flatpak apps
}

// Dir returns the profiles directory (~/.config/faraday/profiles), honoring XDG_CONFIG_HOME.
func Dir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "faraday", "profiles"), nil
}

// validName rejects profile names that could escape the profiles directory
// (path separators, "..", or empty). Names come from the CLI, so this keeps a
// stray `faraday run ../../x` or `faraday save ../x` from reading or writing
// files outside ~/.config/faraday/profiles.
func validName(name string) error {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsRune(name, '/') || strings.ContainsRune(name, filepath.Separator) {
		return fmt.Errorf("invalid profile name %q", name)
	}
	return nil
}

// Load reads a profile by name from the profiles dir.
func Load(name string) (*Profile, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	return LoadFile(filepath.Join(dir, name+".yaml"))
}

// LoadFile reads a profile from an explicit path.
func LoadFile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &p, nil
}

// Save writes a profile as <name>.yaml in the profiles dir.
func Save(name string, p *Profile) error {
	if err := validName(name); err != nil {
		return err
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".yaml"), data, 0o644)
}

// List returns the names (without extension) of available profiles.
func List() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	seen := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		// Collapse a "name.yaml"/"name.yml" pair to a single logical profile so
		// List doesn't report the same name twice.
		name := e.Name()[:len(e.Name())-len(ext)]
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names, nil
}
