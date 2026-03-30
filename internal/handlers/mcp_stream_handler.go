package handlers

import (
	"context"
	"errors"
	"os"
	"strings"

	"aigent/internal/ai"
	"aigent/internal/database"
	"aigent/internal/mcpstream"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// McpStreamConfigHandler CRUD + prueba para servidores MCP HTTP streamable (SSE).
type McpStreamConfigHandler struct {
	Brain   *ai.Brain
	Manager *mcpstream.Manager
}

type mcpStreamRequest struct {
	Alias                string            `json:"alias"`
	BaseURL              string            `json:"base_url"`
	Headers              map[string]string `json:"headers"`
	DisableStandaloneSSE *bool             `json:"disable_standalone_sse"`
	Enabled              *bool             `json:"enabled"`
}

func (h *McpStreamConfigHandler) triggerReloadAndSync() {
	if h.Manager == nil || h.Brain == nil {
		return
	}
	go func() {
		ctx := context.Background()
		h.Manager.ReloadFromDB(ctx)
		_ = h.Brain.SyncTools(ctx)
	}()
}

func (h *McpStreamConfigHandler) List(c *fiber.Ctx) error {
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(masterKey) != 32 {
		return c.Status(500).JSON(fiber.Map{"error": "DB_ENCRYPTION_KEY must be 32 characters"})
	}
	var rows []database.McpStreamServer
	if err := database.DB.Order("id asc").Find(&rows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	out := make([]fiber.Map, 0, len(rows))
	for _, s := range rows {
		headersMasked := map[string]string{}
		if s.HeadersCipher != "" {
			if hdr, err := mcpstream.DecryptHeadersCipher(s.HeadersCipher, masterKey); err == nil {
				for k := range hdr {
					headersMasked[k] = "********"
				}
			}
		}
		out = append(out, fiber.Map{
			"id":                     s.ID,
			"alias":                  s.Alias,
			"base_url":               s.BaseURL,
			"headers":                headersMasked,
			"disable_standalone_sse": s.DisableStandaloneSSE,
			"enabled":                s.Enabled,
			"created_at":             s.CreatedAt,
			"updated_at":             s.UpdatedAt,
		})
	}
	return c.JSON(out)
}

func (h *McpStreamConfigHandler) Create(c *fiber.Ctx) error {
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(masterKey) != 32 {
		return c.Status(500).JSON(fiber.Map{"error": "DB_ENCRYPTION_KEY must be 32 characters"})
	}
	var req mcpStreamRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	req.Alias = strings.TrimSpace(req.Alias)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	if req.Alias == "" || req.BaseURL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "alias and base_url are required"})
	}
	taken, err := database.IsMcpAliasTaken(req.Alias, 0, 0)
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
	disableSSE := false
	if req.DisableStandaloneSSE != nil {
		disableSSE = *req.DisableStandaloneSSE
	}
	cipher, err := mcpstream.EncryptHeadersCipher(req.Headers, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to encrypt headers"})
	}
	row := database.McpStreamServer{
		Alias:                req.Alias,
		BaseURL:              req.BaseURL,
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
	return c.JSON(fiber.Map{"status": "created", "id": row.ID})
}

func (h *McpStreamConfigHandler) Update(c *fiber.Ctx) error {
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(masterKey) != 32 {
		return c.Status(500).JSON(fiber.Map{"error": "DB_ENCRYPTION_KEY must be 32 characters"})
	}
	id := c.Params("id")
	var cur database.McpStreamServer
	if err := database.DB.First(&cur, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	var req mcpStreamRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if a := strings.TrimSpace(req.Alias); a != "" {
		taken, err := database.IsMcpAliasTaken(a, 0, cur.ID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if taken {
			return c.Status(409).JSON(fiber.Map{"error": "alias already exists"})
		}
		cur.Alias = a
	}
	if strings.TrimSpace(req.BaseURL) != "" {
		cur.BaseURL = strings.TrimSpace(req.BaseURL)
	}
	if req.DisableStandaloneSSE != nil {
		cur.DisableStandaloneSSE = *req.DisableStandaloneSSE
	}
	if req.Enabled != nil {
		cur.Enabled = *req.Enabled
	}
	if req.Headers != nil {
		oldMap, _ := mcpstream.DecryptHeadersCipher(cur.HeadersCipher, masterKey)
		if oldMap == nil {
			oldMap = map[string]string{}
		}
		merged := make(map[string]string)
		for k, v := range req.Headers {
			if v == "********" {
				if ov, ok := oldMap[k]; ok {
					merged[k] = ov
				}
			} else if v != "" {
				merged[k] = v
			}
		}
		cipher, err := mcpstream.EncryptHeadersCipher(merged, masterKey)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to encrypt headers"})
		}
		cur.HeadersCipher = cipher
	}
	if err := database.DB.Save(&cur).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	h.triggerReloadAndSync()
	return c.JSON(fiber.Map{"status": "updated"})
}

func (h *McpStreamConfigHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	res := database.DB.Delete(&database.McpStreamServer{}, id)
	if res.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": res.Error.Error()})
	}
	if res.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	}
	h.triggerReloadAndSync()
	return c.JSON(fiber.Map{"status": "deleted"})
}

// TestDryRun prueba URL + headers sin persistir.
func (h *McpStreamConfigHandler) TestDryRun(c *fiber.Ctx) error {
	var req mcpStreamRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if strings.TrimSpace(req.BaseURL) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "base_url is required"})
	}
	disableSSE := false
	if req.DisableStandaloneSSE != nil {
		disableSSE = *req.DisableStandaloneSSE
	}
	names, err := mcpstream.TestConnection(c.Context(), strings.TrimSpace(req.BaseURL), req.Headers, disableSSE)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error(), "tools": nil})
	}
	return c.JSON(fiber.Map{"ok": true, "tools": names})
}

// TestSaved prueba la fila guardada.
func (h *McpStreamConfigHandler) TestSaved(c *fiber.Ctx) error {
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(masterKey) != 32 {
		return c.Status(500).JSON(fiber.Map{"error": "DB_ENCRYPTION_KEY must be 32 characters"})
	}
	id := c.Params("id")
	var row database.McpStreamServer
	if err := database.DB.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	headers, err := mcpstream.DecryptHeadersCipher(row.HeadersCipher, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "headers decrypt failed"})
	}
	names, err := mcpstream.TestConnection(c.Context(), row.BaseURL, headers, row.DisableStandaloneSSE)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error(), "tools": nil})
	}
	return c.JSON(fiber.Map{"ok": true, "tools": names, "alias": row.Alias})
}
