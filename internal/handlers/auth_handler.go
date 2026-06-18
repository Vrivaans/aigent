package handlers

import (
	"errors"
	"log"

	"aigent/internal/auth"
	"aigent/internal/database"

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

	_, err := database.AuthenticateUser(database.DB, req.Username, req.Password)
	if err != nil {
		if errors.Is(err, database.ErrInvalidCredentials) {
			log.Printf("[login] rejected: invalid credentials for user %q", req.Username)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid username or password",
			})
		}
		log.Printf("[login] database error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Login failed",
		})
	}

	token, err := auth.GenerateToken(req.Username)
	if err != nil {
		log.Printf("[login] GenerateToken error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	log.Printf("[login] success: token issued for user %q", req.Username)

	return c.JSON(fiber.Map{
		"token": token,
	})
}
