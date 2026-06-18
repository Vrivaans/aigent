package handlers

import (
	"errors"
	"log"

	"aigent/internal/audit"
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

	user, err := database.AuthenticateUser(database.DB, req.Username, req.Password)
	if err != nil {
		if errors.Is(err, database.ErrInvalidCredentials) {
			log.Printf("[login] rejected: invalid credentials for user %q", req.Username)
			audit.EmitLogin(c, "auth.login.failure", nil, audit.LoginFailurePayload(req.Username, "invalid_credentials"))
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid username or password",
			})
		}
		log.Printf("[login] database error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Login failed",
		})
	}

	roles, err := database.GetUserRoleNames(database.DB, user.ID)
	if err != nil {
		log.Printf("[login] failed to load roles for user %d: %v", user.ID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Login failed",
		})
	}

	token, err := auth.GenerateToken(user.ID, user.Username, roles)
	if err != nil {
		log.Printf("[login] GenerateToken error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	log.Printf("[login] success: token issued for user %q", req.Username)
	actorID := user.ID
	audit.EmitLogin(c, "auth.login.success", &actorID, audit.LoginSuccessPayload(req.Username))

	return c.JSON(fiber.Map{
		"token": token,
	})
}
