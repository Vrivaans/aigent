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

	trimUserOK := strings.TrimSpace(req.Username) == strings.TrimSpace(adminUser)
	trimPassOK := strings.TrimSpace(req.Password) == strings.TrimSpace(adminPass)

	if req.Username != adminUser || req.Password != adminPass {
		log.Printf("[login] rejected: strict user=%v pass=%v trim user=%v pass=%v lens env(usr,pwd)=%d,%d req(usr,pwd)=%d,%d",
			req.Username == adminUser, req.Password == adminPass, trimUserOK, trimPassOK,
			len(adminUser), len(adminPass), len(req.Username), len(req.Password))
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

	log.Printf("[login] success: token issued (user len=%d)", len(req.Username))

	return c.JSON(fiber.Map{
		"token": token,
	})
}
