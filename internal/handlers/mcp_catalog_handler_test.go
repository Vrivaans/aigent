package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aigent/internal/audit"
	"aigent/internal/auth"
	"aigent/internal/database"
	"aigent/internal/handlers"
	"aigent/internal/mcpstdio"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMcpCatalogInstallTest(t *testing.T) *fiber.App {
	t.Helper()
	if err := os.Setenv("DB_ENCRYPTION_KEY", "12345678901234567890123456789012"); err != nil {
		t.Fatalf("setenv: %v", err)
	}

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&database.McpStdioServer{}, &database.McpStreamServer{}, &database.AuditEvent{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	database.DB = db
	restoreAudit := audit.SetDBForTest(db)
	auth.PermissionChecker = func(userID uint, resource, action string) (bool, error) {
		if resource == "mcp" && action == "write" {
			return true, nil
		}
		if resource == "mcp" && action == "read" {
			return true, nil
		}
		return false, nil
	}
	t.Cleanup(func() {
		auth.PermissionChecker = nil
		restoreAudit()
		database.DB = nil
		_ = os.Unsetenv("DB_ENCRYPTION_KEY")
	})

	catalogDir := t.TempDir()
	manifest := map[string]interface{}{
		"id":            "filesystem",
		"name":          "Filesystem",
		"description":   "test manifest",
		"version":       "1.0.0",
		"transport":     "stdio",
		"default_alias": "filesystem",
		"param_defaults": map[string]string{
			"allowed_dir": ".",
		},
		"stdio": map[string]interface{}{
			"command": "npx",
			"args":    []string{"-y", "@modelcontextprotocol/server-filesystem", "{{allowed_dir}}"},
			"env":     map[string]string{},
		},
	}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(catalogDir, "filesystem.json"), raw, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	handler := &handlers.McpCatalogHandler{
		StdioMgr:   mcpstdio.NewManager(),
		CatalogDir: catalogDir,
	}

	app := fiber.New()
	app.Use(audit.CorrelationMiddleware())
	app.Use(func(c *fiber.Ctx) error {
		auth.SetRequestUser(c, &auth.Claims{UserID: 1, Username: "operator", Roles: []string{"operator"}})
		return c.Next()
	})
	app.Post("/api/catalog/mcp/install", auth.RequirePermissionMiddleware("mcp", "write"), handler.Install)
	app.Get("/api/catalog/mcp", auth.RequirePermissionMiddleware("mcp", "read"), handler.List)

	return app
}

func TestMcpCatalogInstallFilesystem(t *testing.T) {
	app := setupMcpCatalogInstallTest(t)

	body, _ := json.Marshal(map[string]interface{}{
		"manifest_id": "filesystem",
		"params":      map[string]string{"allowed_dir": "/tmp/aigent-test"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/catalog/mcp/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d body=%s", resp.StatusCode, readMcpCatalogBody(t, resp))
	}

	var row database.McpStdioServer
	if err := database.DB.Where("alias = ?", "filesystem").First(&row).Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	args, err := mcpstdio.ParseArgsJSON(row.ArgsJSON)
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if row.Command != "npx" || len(args) != 3 || args[2] != "/tmp/aigent-test" {
		t.Fatalf("unexpected stdio config: command=%q args=%v", row.Command, args)
	}
}

func TestMcpCatalogListTemplates(t *testing.T) {
	app := setupMcpCatalogInstallTest(t)
	req := httptest.NewRequest(http.MethodGet, "/api/catalog/mcp", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d body=%s", resp.StatusCode, readMcpCatalogBody(t, resp))
	}
}

func TestMcpCatalogInstallRejectsInvalidManifestID(t *testing.T) {
	app := setupMcpCatalogInstallTest(t)

	body, _ := json.Marshal(map[string]string{"manifest_id": "missing"})
	req := httptest.NewRequest(http.MethodPost, "/api/catalog/mcp/install", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func readMcpCatalogBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}
