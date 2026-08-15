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

	c.Set("curl@8.0.0", "Curl is a CLI tool")
	c.Set("neovim", "Neovim is a text editor")

	// Match by name@version
	if text, ok := c.GetPackage("curl", "8.0.0"); !ok || text != "Curl is a CLI tool" {
		t.Errorf("expected curl@8.0.0 match, got %v, ok=%v", text, ok)
	}

	// Match by name fallback
	if text, ok := c.GetPackage("neovim", "0.10.0"); !ok || text != "Neovim is a text editor" {
		t.Errorf("expected neovim name fallback match, got %v, ok=%v", text, ok)
	}

	// Check Has
	if !c.Has("curl", "8.0.0") {
		t.Errorf("expected Has('curl', '8.0.0') == true")
	}

	if !c.Has("neovim", "0.10.0") {
		t.Errorf("expected Has('neovim', '0.10.0') == true")
	}

	if c.Has("nonexistent", "1.0.0") {
		t.Errorf("expected Has('nonexistent', '1.0.0') == false")
	}

	// Verify persistence file exists
	if _, err := os.Stat(c.Path()); os.IsNotExist(err) {
		t.Errorf("expected analysis.json file to exist at %s", c.Path())
	}
}
