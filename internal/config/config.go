// Package config holds the application settings.
//
// Settings live in the database as name and value text pairs and are cached
// in memory so that a request does not have to query for each one it reads.
// The cache is refreshed at the start of every request, which is what lets a
// setting changed through the admin UI take effect in the other processes.
package config

import (
	"context"
	"maps"
	"strconv"
	"strings"
	"sync"

	"github.com/jeffmvr/mangle-vpn/internal/store"
)

// Config is a cached view of the application settings. It is safe for
// concurrent use.
type Config struct {
	settings *store.SettingStore

	mu     sync.RWMutex
	values map[string]string
}

// New returns a Config backed by the given setting store. Call [Config.Reload]
// before reading from it.
func New(settings *store.SettingStore) *Config {
	return &Config{settings: settings, values: make(map[string]string)}
}

// Reload refreshes the cache from the database.
func (c *Config) Reload(ctx context.Context) error {
	values, err := c.settings.All(ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = values
	return nil
}

// All returns a copy of every setting.
func (c *Config) All() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return maps.Clone(c.values)
}

// Has reports whether the setting exists.
func (c *Config) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.values[name]
	return ok
}

// Get returns the value of a setting, or an empty string when it is unset.
func (c *Config) Get(name string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.values[name]
}

// GetOr returns the value of a setting, falling back to fallback when it is
// unset or empty.
func (c *Config) GetOr(name, fallback string) string {
	if value := c.Get(name); value != "" {
		return value
	}
	return fallback
}

// Int returns the value of a setting as an integer, falling back to fallback
// when it is unset or not a number.
func (c *Config) Int(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.Get(name)))
	if err != nil {
		return fallback
	}
	return value
}

// Bool returns the value of a setting as a boolean, falling back to fallback
// when it is unset. Any of "true", "yes", "1", and "on" counts as true,
// whatever their case; anything else is false.
func (c *Config) Bool(name string, fallback bool) bool {
	value := c.Get(name)
	if value == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1", "on":
		return true
	default:
		return false
	}
}

// Lines returns the value of a setting split into its non-empty lines. It
// suits the settings that hold a list, such as routes and nameservers.
func (c *Config) Lines(name string) []string {
	return SplitLines(c.Get(name))
}

// Set stores the value of a setting and updates the cache.
func (c *Config) Set(ctx context.Context, name, value string) error {
	if err := c.settings.Set(ctx, name, value); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[name] = value
	return nil
}

// SetBool stores the value of a boolean setting. Booleans are written as
// "True" and "False" so that a database shared with the Django release reads
// the same either way.
func (c *Config) SetBool(ctx context.Context, name string, value bool) error {
	if value {
		return c.Set(ctx, name, "True")
	}
	return c.Set(ctx, name, "False")
}

// SetDefault stores the value of a setting only when it does not yet exist.
func (c *Config) SetDefault(ctx context.Context, name, value string) error {
	if c.Has(name) {
		return nil
	}
	return c.Set(ctx, name, value)
}

// Delete removes a setting.
func (c *Config) Delete(ctx context.Context, name string) error {
	if err := c.settings.Delete(ctx, name); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.values, name)
	return nil
}

// Organization returns the organization name shown in the UI and in e-mail.
func (c *Config) Organization() string {
	return c.GetOr(AppOrganization, "Mangle")
}

// URL returns an absolute URL to the web application, with the given path
// segments appended.
func (c *Config) URL(paths ...string) string {
	var b strings.Builder
	b.WriteString("https://")
	b.WriteString(c.Get(AppHostname))

	// Only a non-standard port needs naming.
	if port := c.Int(AppHTTPSPort, 443); port != 443 {
		b.WriteString(":")
		b.WriteString(strconv.Itoa(port))
	}

	b.WriteString("/")
	b.WriteString(strings.Join(paths, "/"))

	return strings.ToLower(b.String())
}

// SplitLines splits a multi-line setting into its lines, trimmed, dropping
// blank ones. Routes and DNS servers are stored this way, one per line.
func SplitLines(value string) []string {
	var out []string
	for line := range strings.SplitSeq(value, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
