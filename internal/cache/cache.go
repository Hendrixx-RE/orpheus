// Package cache defines how the already analysed packages are stored
package cache

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
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
	dir = filepath.Join(dir, "orpheus")
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
		log.Fatal(err)
	}
}

func (c *Cache) save() {
	c.mu.RLock()
	data, err := json.MarshalIndent(c.data, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return
	}
	if err := os.WriteFile(c.path, data, 0o644); err != nil {
		log.Fatal(err)
	}
}
