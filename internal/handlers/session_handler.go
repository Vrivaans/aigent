package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"aigent/internal/database"
	"aigent/internal/rag"

	"github.com/gofiber/fiber/v2"
)

// UpdateSessionGoals updates the goals of a session.
// POST /api/sessions/:id/goals
func UpdateSessionGoals(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	
	var req struct {
		Goals string `json:"goals"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	session, err := loadSessionForTenant(c, sessionID)
	if err != nil {
		return respondFiberError(c, err)
	}

	session.SessionGoals = req.Goals
	if err := database.DB.Save(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update session goals"})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"goals":  session.SessionGoals,
	})
}

// UpdateSessionWorkspace updates the local workspace path of a session.
// POST /api/sessions/:id/workspace
func UpdateSessionWorkspace(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	
	var req struct {
		WorkspacePath string `json:"workspace_path"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	session, err := loadSessionForTenant(c, sessionID)
	if err != nil {
		return respondFiberError(c, err)
	}

	session.WorkspacePath = req.WorkspacePath
	if err := database.DB.Save(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update session workspace path"})
	}

	return c.JSON(fiber.Map{
		"status":         "success",
		"workspace_path": session.WorkspacePath,
	})
}

// UploadSessionFile uploads a file specifically for a session context (Layer 2 cache).
// POST /api/sessions/:id/files
func UploadSessionFile(c *fiber.Ctx) error {
	sessionIDStr := c.Params("id")
	sessionID, err := strconv.Atoi(sessionIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid session ID"})
	}

	// 1. Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No file uploaded"})
	}

	// 2. Open file stream
	fileStream, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open uploaded file"})
	}
	defer fileStream.Close()

	if _, err := loadSessionForTenant(c, sessionIDStr); err != nil {
		return respondFiberError(c, err)
	}

	// 3. Convert to Markdown text and compute hash
	markdown, hash, err := rag.ConvertFileToMarkdown(fileStream, file.Filename)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Failed to convert file to Markdown: %v", err)})
	}

	// 4. Save SessionFile
	sessionFile := database.SessionFile{
		SessionID: uint(sessionID),
		Filename:  file.Filename,
		Content:   markdown,
		Hash:      hash,
		CreatedAt: time.Now(),
	}

	if err := database.DB.Create(&sessionFile).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save session file to database"})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": fmt.Sprintf("Successfully processed and added context file: %s", file.Filename),
		"file":    sessionFile,
	})
}

// GetSessionFiles lists context files uploaded to a session.
// GET /api/sessions/:id/files
func GetSessionFiles(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	if _, err := loadSessionForTenant(c, sessionID); err != nil {
		return respondFiberError(c, err)
	}

	var files []database.SessionFile
	// Seleccionar campos ligeros para listar, excluyendo el content que puede ser pesado.
	if err := database.DB.Select("id, session_id, filename, hash, created_at").
		Where("session_id = ?", sessionID).
		Order("filename asc").
		Find(&files).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(files)
}

// DeleteSessionFile deletes a context file from a session.
// DELETE /api/sessions/:id/files/:file_id
func DeleteSessionFile(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	fileID := c.Params("file_id")

	if _, err := loadSessionForTenant(c, sessionID); err != nil {
		return respondFiberError(c, err)
	}

	var sessionFile database.SessionFile
	if err := database.DB.Where("session_id = ? AND id = ?", sessionID, fileID).First(&sessionFile).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "File not found in this session"})
	}

	if err := database.DB.Delete(&sessionFile).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete file"})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Session file deleted successfully",
	})
}

// BrowseWorkspaceDirectories lists subdirectories of a given path.
// GET /api/workspace/browse
func BrowseWorkspaceDirectories(c *fiber.Ctx) error {
	pathQuery := c.Query("path")

	// If no path is provided, default to user's home directory.
	if pathQuery == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			cwd, err := os.Getwd()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get current directory"})
			}
			pathQuery = cwd
		} else {
			pathQuery = homeDir
		}
	}

	// Clean the path
	cleanPath := filepath.Clean(pathQuery)

	// Open the directory
	dir, err := os.Open(cleanPath)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Failed to open directory: %v", err)})
	}
	defer dir.Close()

	// Read entry info
	entries, err := dir.Readdir(-1)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Failed to read directory contents: %v", err)})
	}

	var subdirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			if name != "." && name != ".." {
				subdirs = append(subdirs, name)
			}
		}
	}

	sort.Strings(subdirs)

	parentPath := filepath.Dir(cleanPath)

	return c.JSON(fiber.Map{
		"current_path": cleanPath,
		"parent_path":  parentPath,
		"directories":  subdirs,
	})
}

