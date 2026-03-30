package handlers

import (
	"log"
	"os"
	"strings"

	"aigent/internal/auth"

	"github.com/gofiber/fiber/v2"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func HandleLogin(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("[login] BodyParser error: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	adminUser := os.Getenv("ADMIN_USERNAME")
	adminPass := os.Getenv("ADMIN_PASSWORD")

	// Debug temporal: quitar o sanitizar antes de producción estable
	log.Printf("[login] env ADMIN_USERNAME=%q (len=%d) ADMIN_PASSWORD=%q (len=%d)",
		adminUser, len(adminUser), adminPass, len(adminPass))
	log.Printf("[login] req username=%q (len=%d) password=%q (len=%d)",
		req.Username, len(req.Username), req.Password, len(req.Password))
	log.Printf("[login] trim compare: user match=%v pass match=%v",
		strings.TrimSpace(req.Username) == strings.TrimSpace(adminUser),
		strings.TrimSpace(req.Password) == strings.TrimSpace(adminPass))
	log.Printf("[login] strict compare: user match=%v pass match=%v",
		req.Username == adminUser, req.Password == adminPass)

	if req.Username != adminUser || req.Password != adminPass {
		log.Printf("[login] rejected: invalid credentials (ver respuesta 401)")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid username or password",
		})
	}

	token, err := auth.GenerateToken(req.Username)
	if err != nil {
		log.Printf("[login] GenerateToken error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	log.Printf("[login] ok: token issued for user=%q", req.Username)

	return c.JSON(fiber.Map{
		"token": token,
	})
}
