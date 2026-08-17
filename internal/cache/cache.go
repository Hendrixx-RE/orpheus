// Package cache defines how the already analysed packages are stored
package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Cache struct {
	mu   sync.RWMutex
	data map[string]string
	path string
}

func New() (*Cache, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "pacseer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	c := &Cache{
		data: make(map[string]string),
		path: filepath.Join(dir, "analysis.json"),
	}
	c.load()
	return c, nil
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

// GetPackage retrieves analysis text checking "name" first, with legacy fallback to "name@version".
func (c *Cache) GetPackage(name, version string) (string, bool) {
	c.mu.RLock()
	if v, ok := c.data[name]; ok {
		c.mu.RUnlock()
		return v, true
	}
	if version != "" {
		if v, ok := c.data[name+"@"+version]; ok {
			c.mu.RUnlock()
			// Promote legacy key to name-level cache
			c.Set(name, v)
			return v, true
		}
	}
	c.mu.RUnlock()
	return "", false
}

// Has checks if analysis text for a package exists under "name" or legacy "name@version".
func (c *Cache) Has(name, version string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.data[name]; ok {
		return true
	}
	if version != "" {
		if _, ok := c.data[name+"@"+version]; ok {
			return true
		}
	}
	return false
}

func (c *Cache) Path() string {
	return c.path
}

func (c *Cache) Set(key, value string) {
	c.mu.Lock()
	c.data[key] = value
	c.mu.Unlock()
	c.save()
}

func (c *Cache) load() {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &c.data); err != nil {
		return
	}
	// Migrate any legacy "name@version" keys to "name"
	migrated := false
	for k, v := range c.data {
		if strings.HasPrefix(k, "summary@") {
			continue
		}
		if idx := strings.Index(k, "@"); idx != -1 {
			name := k[:idx]
			if _, exists := c.data[name]; !exists {
				c.data[name] = v
				migrated = true
			}
		}
	}
	if migrated {
		c.save()
	}
}

func (c *Cache) save() {
	c.mu.RLock()
	data, err := json.MarshalIndent(c.data, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return
	}
	_ = os.WriteFile(c.path, data, 0o644)
}
