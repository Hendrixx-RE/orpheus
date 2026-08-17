package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheGetPackageAndHas(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{
		data: make(map[string]string),
		path: filepath.Join(dir, "analysis.json"),
	}

	// Store by package name
	c.Set("neovim", "Neovim is a text editor")
	c.Set("curl@8.0.0", "Curl is a CLI tool")

	// Match by name directly across any version
	if text, ok := c.GetPackage("neovim", "0.10.4"); !ok || text != "Neovim is a text editor" {
		t.Errorf("expected neovim match, got %v, ok=%v", text, ok)
	}
	if text, ok := c.GetPackage("neovim", "0.11.0"); !ok || text != "Neovim is a text editor" {
		t.Errorf("expected neovim version upgrade match, got %v, ok=%v", text, ok)
	}

	// Match legacy name@version and auto-promote to name
	if text, ok := c.GetPackage("curl", "8.0.0"); !ok || text != "Curl is a CLI tool" {
		t.Errorf("expected legacy curl@8.0.0 match, got %v, ok=%v", text, ok)
	}
	if text, ok := c.GetPackage("curl", "8.1.0"); !ok || text != "Curl is a CLI tool" {
		t.Errorf("expected promoted curl match on newer version, got %v, ok=%v", text, ok)
	}

	// Check Has
	if !c.Has("neovim", "0.10.4") {
		t.Errorf("expected Has('neovim', '0.10.4') == true")
	}
	if !c.Has("curl", "8.1.0") {
		t.Errorf("expected Has('curl', '8.1.0') == true")
	}
	if c.Has("nonexistent", "1.0.0") {
		t.Errorf("expected Has('nonexistent', '1.0.0') == false")
	}

	// Verify persistence file exists
	if _, err := os.Stat(c.Path()); os.IsNotExist(err) {
		t.Errorf("expected analysis.json file to exist at %s", c.Path())
	}
}
