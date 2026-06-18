package mcpcatalog_test

import (
	"os"
	"path/filepath"
	"testing"

	"aigent/internal/mcpcatalog"
)

func TestValidateManifestStdioOK(t *testing.T) {
	m := &mcpcatalog.Manifest{
		ID:           "filesystem",
		Name:         "Filesystem",
		Version:      "1.0.0",
		Transport:    mcpcatalog.TransportStdio,
		DefaultAlias: "filesystem",
		Stdio: &mcpcatalog.StdioSpec{
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "{{allowed_dir}}"},
			Env:     map[string]string{},
		},
	}
	if err := mcpcatalog.ValidateManifest(m); err != nil {
		t.Fatalf("ValidateManifest: %v", err)
	}
}

func TestValidateManifestStreamOK(t *testing.T) {
	m := &mcpcatalog.Manifest{
		ID:           "playwright",
		Name:         "Playwright",
		Version:      "1.0.0",
		Transport:    mcpcatalog.TransportStream,
		DefaultAlias: "playwright",
		Stream: &mcpcatalog.StreamSpec{
			BaseURL: "http://127.0.0.1:8931/mcp",
		},
	}
	if err := mcpcatalog.ValidateManifest(m); err != nil {
		t.Fatalf("ValidateManifest: %v", err)
	}
}

func TestValidateManifestRejectsMissingStdioBlock(t *testing.T) {
	m := &mcpcatalog.Manifest{
		ID:        "bad",
		Name:      "Bad",
		Version:   "1.0.0",
		Transport: mcpcatalog.TransportStdio,
	}
	if err := mcpcatalog.ValidateManifest(m); err == nil {
		t.Fatal("expected error for missing stdio block")
	}
}

func TestValidateManifestRejectsInvalidTransport(t *testing.T) {
	m := &mcpcatalog.Manifest{
		ID:        "bad",
		Name:      "Bad",
		Version:   "1.0.0",
		Transport: "websocket",
	}
	if err := mcpcatalog.ValidateManifest(m); err == nil {
		t.Fatal("expected error for invalid transport")
	}
}

func TestParseManifestBundledFilesystem(t *testing.T) {
	dir := filepath.Join("..", "..", "catalog", "mcp")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("catalog/mcp not available from test cwd")
	}
	c := mcpcatalog.NewCatalog(dir)
	m, err := c.LoadByID("filesystem")
	if err != nil {
		t.Fatalf("LoadByID(filesystem): %v", err)
	}
	if m.Transport != mcpcatalog.TransportStdio {
		t.Fatalf("transport = %q", m.Transport)
	}
	cmd, args, env, err := m.ResolvedStdioConfig(map[string]string{"allowed_dir": "/tmp"})
	if err != nil {
		t.Fatalf("ResolvedStdioConfig: %v", err)
	}
	if cmd != "npx" {
		t.Fatalf("command = %q", cmd)
	}
	if len(args) != 3 || args[2] != "/tmp" {
		t.Fatalf("args = %v", args)
	}
	if env == nil {
		t.Fatal("env should not be nil")
	}
}

func TestResolvedStdioConfigRequiresParams(t *testing.T) {
	m := &mcpcatalog.Manifest{
		ID:        "filesystem",
		Name:      "Filesystem",
		Version:   "1.0.0",
		Transport: mcpcatalog.TransportStdio,
		Stdio: &mcpcatalog.StdioSpec{
			Command: "npx",
			Args:    []string{"{{allowed_dir}}"},
		},
	}
	if _, _, _, err := m.ResolvedStdioConfig(nil); err == nil {
		t.Fatal("expected missing param error")
	}
}

func TestCatalogLoadByIDMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "filesystem.json")
	if err := os.WriteFile(path, []byte(`{
		"id": "other",
		"name": "Filesystem",
		"version": "1.0.0",
		"transport": "stdio",
		"stdio": {"command": "npx", "args": []}
	}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	c := mcpcatalog.NewCatalog(dir)
	if _, err := c.LoadByID("filesystem"); err == nil {
		t.Fatal("expected id mismatch error")
	}
}
