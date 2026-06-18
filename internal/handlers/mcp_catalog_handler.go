package handlers

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"aigent/internal/ai"
	"aigent/internal/audit"
	"aigent/internal/database"
	"aigent/internal/mcpcatalog"
	"aigent/internal/mcpstdio"
	"aigent/internal/mcpstream"
	"aigent/internal/secrets"

	"github.com/gofiber/fiber/v2"
)

// McpCatalogHandler installs bundled MCP templates from catalog manifests.
type McpCatalogHandler struct {
	Brain      *ai.Brain
	StdioMgr   *mcpstdio.Manager
	StreamMgr  *mcpstream.Manager
	CatalogDir string
}

type mcpCatalogInstallRequest struct {
	ManifestID string            `json:"manifest_id"`
	Alias      string            `json:"alias"`
	Params     map[string]string `json:"params"`
	Enabled    *bool             `json:"enabled"`
}

var catalogAliasPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func (h *McpCatalogHandler) catalog() *mcpcatalog.Catalog {
	return mcpcatalog.NewCatalog(h.CatalogDir)
}

func (h *McpCatalogHandler) triggerReloadAndSync() {
	if h.Brain == nil {
		return
	}
	go func() {
		ctx := context.Background()
		if h.StdioMgr != nil {
			h.StdioMgr.ReloadFromDB(ctx)
		}
		if h.StreamMgr != nil {
			h.StreamMgr.ReloadFromDB(ctx)
		}
		_ = h.Brain.SyncTools(ctx)
	}()
}

// Install reads a catalog manifest and creates an MCP stdio or stream entry.
func (h *McpCatalogHandler) Install(c *fiber.Ctx) error {
	masterKey, err := secrets.RequireDBEncryptionKey()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	var req mcpCatalogInstallRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	req.ManifestID = strings.TrimSpace(req.ManifestID)
	if req.ManifestID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "manifest_id is required"})
	}

	m, err := h.catalog().LoadByID(req.ManifestID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	alias := m.ResolveAlias(req.Alias)
	if !catalogAliasPattern.MatchString(alias) {
		return c.Status(400).JSON(fiber.Map{"error": "alias has invalid format"})
	}

	taken, err := database.IsMcpAliasTaken(alias, 0, 0)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if taken {
		return c.Status(409).JSON(fiber.Map{"error": "alias already exists"})
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	switch m.Transport {
	case mcpcatalog.TransportStdio:
		return h.installStdio(c, masterKey, m, alias, req.Params, enabled)
	case mcpcatalog.TransportStream:
		return h.installStream(c, masterKey, m, alias, req.Params, enabled)
	default:
		return c.Status(400).JSON(fiber.Map{"error": "unsupported transport"})
	}
}

func (h *McpCatalogHandler) installStdio(c *fiber.Ctx, masterKey string, m *mcpcatalog.Manifest, alias string, params map[string]string, enabled bool) error {
	command, args, env, err := m.ResolvedStdioConfig(params)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid resolved args"})
	}
	cipher, err := mcpstdio.EncryptEnvCipher(env, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to encrypt env"})
	}
	row := database.McpStdioServer{
		Alias:     alias,
		Command:   command,
		ArgsJSON:  argsJSON,
		EnvCipher: cipher,
		Enabled:   enabled,
	}
	if err := database.DB.Create(&row).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE") {
			return c.Status(409).JSON(fiber.Map{"error": "alias already exists"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	h.triggerReloadAndSync()
	audit.Emit(c, audit.Event{
		Action:       "mcp_catalog.install",
		ResourceType: "mcp_stdio",
		ResourceID:   audit.UintID(row.ID),
		PayloadAfter: auditJSON(fiber.Map{
			"manifest_id": m.ID,
			"alias":       alias,
			"transport":   m.Transport,
		}),
	})
	return c.JSON(fiber.Map{
		"status":      "installed",
		"transport":   m.Transport,
		"manifest_id": m.ID,
		"alias":       alias,
		"id":          row.ID,
	})
}

func (h *McpCatalogHandler) installStream(c *fiber.Ctx, masterKey string, m *mcpcatalog.Manifest, alias string, params map[string]string, enabled bool) error {
	baseURL, headers, disableSSE, err := m.ResolvedStreamConfig(params)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	cipher, err := mcpstream.EncryptHeadersCipher(headers, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to encrypt headers"})
	}
	row := database.McpStreamServer{
		Alias:                alias,
		BaseURL:              baseURL,
		HeadersCipher:        cipher,
		DisableStandaloneSSE: disableSSE,
		Enabled:              enabled,
	}
	if err := database.DB.Create(&row).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE") {
			return c.Status(409).JSON(fiber.Map{"error": "alias already exists"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	h.triggerReloadAndSync()
	audit.Emit(c, audit.Event{
		Action:       "mcp_catalog.install",
		ResourceType: "mcp_stream",
		ResourceID:   audit.UintID(row.ID),
		PayloadAfter: auditJSON(fiber.Map{
			"manifest_id": m.ID,
			"alias":       alias,
			"transport":   m.Transport,
		}),
	})
	return c.JSON(fiber.Map{
		"status":      "installed",
		"transport":   m.Transport,
		"manifest_id": m.ID,
		"alias":       alias,
		"id":          row.ID,
	})
}

func auditJSON(v fiber.Map) *string {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}
