package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"aigent/internal/ai"
	"aigent/internal/database"
	"aigent/internal/mcpstdio"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// McpStdioConfigHandler CRUD + prueba de conexión para servidores MCP stdio.
type McpStdioConfigHandler struct {
	Brain   *ai.Brain
	Manager *mcpstdio.Manager
}

type mcpStdioRequest struct {
	Alias   string            `json:"alias"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Enabled *bool             `json:"enabled"`
}

func (h *McpStdioConfigHandler) triggerReloadAndSync() {
	if h.Manager == nil || h.Brain == nil {
		return
	}
	go func() {
		ctx := context.Background()
		h.Manager.ReloadFromDB(ctx)
		_ = h.Brain.SyncTools(ctx)
	}()
}

func (h *McpStdioConfigHandler) List(c *fiber.Ctx) error {
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(masterKey) != 32 {
		return c.Status(500).JSON(fiber.Map{"error": "DB_ENCRYPTION_KEY must be 32 characters"})
	}
	var rows []database.McpStdioServer
	if err := database.DB.Order("id asc").Find(&rows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	out := make([]fiber.Map, 0, len(rows))
	for _, s := range rows {
		args, _ := mcpstdio.ParseArgsJSON(s.ArgsJSON)
		envMasked := map[string]string{}
		if s.EnvCipher != "" {
			if env, err := mcpstdio.DecryptEnvCipher(s.EnvCipher, masterKey); err == nil {
				for k := range env {
					envMasked[k] = "********"
				}
			}
		}
		out = append(out, fiber.Map{
			"id":         s.ID,
			"alias":      s.Alias,
			"command":    s.Command,
			"args":       args,
			"env":        envMasked,
			"enabled":    s.Enabled,
			"created_at": s.CreatedAt,
			"updated_at": s.UpdatedAt,
		})
	}
	return c.JSON(out)
}

func (h *McpStdioConfigHandler) Create(c *fiber.Ctx) error {
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(masterKey) != 32 {
		return c.Status(500).JSON(fiber.Map{"error": "DB_ENCRYPTION_KEY must be 32 characters"})
	}
	var req mcpStdioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	req.Alias = strings.TrimSpace(req.Alias)
	if req.Alias == "" || strings.TrimSpace(req.Command) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "alias and command are required"})
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	argsJSON, err := json.Marshal(req.Args)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid args"})
	}
	cipher, err := mcpstdio.EncryptEnvCipher(req.Env, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to encrypt env"})
	}
	row := database.McpStdioServer{
		Alias:     req.Alias,
		Command:   strings.TrimSpace(req.Command),
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
	return c.JSON(fiber.Map{"status": "created", "id": row.ID})
}

func (h *McpStdioConfigHandler) Update(c *fiber.Ctx) error {
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(masterKey) != 32 {
		return c.Status(500).JSON(fiber.Map{"error": "DB_ENCRYPTION_KEY must be 32 characters"})
	}
	id := c.Params("id")
	var cur database.McpStdioServer
	if err := database.DB.First(&cur, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	var req mcpStdioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if a := strings.TrimSpace(req.Alias); a != "" {
		var n int64
		_ = database.DB.Model(&database.McpStdioServer{}).Where("alias = ? AND id <> ?", a, cur.ID).Count(&n)
		if n > 0 {
			return c.Status(409).JSON(fiber.Map{"error": "alias already exists"})
		}
		cur.Alias = a
	}
	if strings.TrimSpace(req.Command) != "" {
		cur.Command = strings.TrimSpace(req.Command)
	}
	if req.Args != nil {
		b, err := json.Marshal(req.Args)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid args"})
		}
		cur.ArgsJSON = b
	}
	if req.Enabled != nil {
		cur.Enabled = *req.Enabled
	}
	if req.Env != nil {
		oldMap, _ := mcpstdio.DecryptEnvCipher(cur.EnvCipher, masterKey)
		if oldMap == nil {
			oldMap = map[string]string{}
		}
		merged := make(map[string]string)
		for k, v := range req.Env {
			if v == "********" {
				if ov, ok := oldMap[k]; ok {
					merged[k] = ov
				}
			} else if v != "" {
				merged[k] = v
			}
		}
		cipher, err := mcpstdio.EncryptEnvCipher(merged, masterKey)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to encrypt env"})
		}
		cur.EnvCipher = cipher
	}
	if err := database.DB.Save(&cur).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	h.triggerReloadAndSync()
	return c.JSON(fiber.Map{"status": "updated"})
}

func (h *McpStdioConfigHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	res := database.DB.Delete(&database.McpStdioServer{}, id)
	if res.Error != nil {
		return c.Status(500).JSON(fiber.Map{"error": res.Error.Error()})
	}
	if res.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "not found"})
	}
	h.triggerReloadAndSync()
	return c.JSON(fiber.Map{"status": "deleted"})
}

// TestDryRun body: command, args, env — sin persistir.
func (h *McpStdioConfigHandler) TestDryRun(c *fiber.Ctx) error {
	var req mcpStdioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if strings.TrimSpace(req.Command) == "" {
		return c.Status(400).JSON(fiber.Map{"error": "command is required"})
	}
	names, err := mcpstdio.TestConnection(c.Context(), strings.TrimSpace(req.Command), req.Args, req.Env)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error(), "tools": nil})
	}
	return c.JSON(fiber.Map{"ok": true, "tools": names})
}

func (h *McpStdioConfigHandler) TestSaved(c *fiber.Ctx) error {
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	if len(masterKey) != 32 {
		return c.Status(500).JSON(fiber.Map{"error": "DB_ENCRYPTION_KEY must be 32 characters"})
	}
	id := c.Params("id")
	var row database.McpStdioServer
	if err := database.DB.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{"error": "not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	args, err := mcpstdio.ParseArgsJSON(row.ArgsJSON)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid stored args"})
	}
	env, err := mcpstdio.DecryptEnvCipher(row.EnvCipher, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "env decrypt failed"})
	}
	names, err := mcpstdio.TestConnection(c.Context(), row.Command, args, env)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error(), "tools": nil})
	}
	return c.JSON(fiber.Map{"ok": true, "tools": names, "alias": row.Alias})
}
