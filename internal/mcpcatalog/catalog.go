package mcpcatalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultCatalogDir returns the directory containing bundled MCP manifests.
func DefaultCatalogDir() string {
	if d := os.Getenv("CATALOG_MCP_DIR"); d != "" {
		return d
	}
	return "catalog/mcp"
}

// Catalog loads manifests from a directory of *.json files.
type Catalog struct {
	Dir string
}

// NewCatalog returns a catalog reader for dir (or DefaultCatalogDir when empty).
func NewCatalog(dir string) *Catalog {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultCatalogDir()
	}
	return &Catalog{Dir: dir}
}

// ListIDs returns manifest ids sorted alphabetically.
func (c *Catalog) ListIDs() ([]string, error) {
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		return nil, fmt.Errorf("read catalog dir %q: %w", c.Dir, err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
	}
	return ids, nil
}

// LoadByID reads and validates `<id>.json` from the catalog directory.
func (c *Catalog) LoadByID(id string) (*Manifest, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("manifest id is required")
	}
	if !idPattern.MatchString(id) {
		return nil, fmt.Errorf("manifest id %q has invalid format", id)
	}
	path := filepath.Join(c.Dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("manifest %q not found", id)
		}
		return nil, fmt.Errorf("read manifest %q: %w", id, err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("manifest %q: %w", id, err)
	}
	if m.ID != id {
		return nil, fmt.Errorf("manifest id %q does not match file name %q", m.ID, id)
	}
	return m, nil
}

// ListManifests loads and validates all manifests in the catalog directory.
func (c *Catalog) ListManifests() ([]PublicView, error) {
	ids, err := c.ListIDs()
	if err != nil {
		return nil, err
	}
	out := make([]PublicView, 0, len(ids))
	for _, id := range ids {
		m, err := c.LoadByID(id)
		if err != nil {
			return nil, err
		}
		out = append(out, m.PublicView())
	}
	return out, nil
}
