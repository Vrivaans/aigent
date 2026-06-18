package handlers

import (
	"errors"
	"strings"

	"aigent/internal/auth"
	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type UserHandler struct{}

type UserResponse struct {
	ID        uint     `json:"id"`
	Username  string   `json:"username"`
	IsActive  bool     `json:"is_active"`
	Roles     []string `json:"roles"`
	CreatedAt string   `json:"created_at"`
}

type CreateUserRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	IsActive *bool    `json:"is_active"`
	Roles    []string `json:"roles"`
}

type UpdateUserRequest struct {
	IsActive *bool   `json:"is_active"`
	Password *string `json:"password"`
}

type UpdateUserRolesRequest struct {
	Roles []string `json:"roles"`
}

func toUserResponse(user database.User, roleNames []string) UserResponse {
	if roleNames == nil {
		roleNames = []string{}
	}
	return UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		IsActive:  user.IsActive,
		Roles:     roleNames,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func loadUserRoleNames(userID uint) ([]string, error) {
	return database.GetUserRoleNames(database.DB, userID)
}

func (h *UserHandler) GetUsers(c *fiber.Ctx) error {
	var users []database.User
	if err := database.DB.Order("username asc").Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	resp := make([]UserResponse, 0, len(users))
	for _, user := range users {
		roles, err := loadUserRoleNames(user.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		resp = append(resp, toUserResponse(user, roles))
	}
	return c.JSON(resp)
}

func (h *UserHandler) CreateUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	username := strings.TrimSpace(req.Username)
	password := req.Password
	if username == "" || password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username and password are required"})
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	user := database.User{
		Username:     username,
		PasswordHash: hash,
		IsActive:     isActive,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to create user (username may already exist)"})
	}

	roles := req.Roles
	if len(roles) == 0 {
		roles = []string{"viewer"}
	}
	if err := database.SetUserRoles(database.DB, user.ID, roles); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	roleNames, _ := loadUserRoleNames(user.ID)
	return c.Status(fiber.StatusCreated).JSON(toUserResponse(user, roleNames))
}

func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var user database.User
	if err := database.DB.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Password != nil && strings.TrimSpace(*req.Password) != "" {
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
		}
		user.PasswordHash = hash
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	roleNames, _ := loadUserRoleNames(user.ID)
	return c.JSON(toUserResponse(user, roleNames))
}

func (h *UserHandler) UpdateUserRoles(c *fiber.Ctx) error {
	id := c.Params("id")
	var user database.User
	if err := database.DB.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var req UpdateUserRolesRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if len(req.Roles) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "roles must not be empty"})
	}

	if err := database.SetUserRoles(database.DB, user.ID, req.Roles); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	roleNames, _ := loadUserRoleNames(user.ID)
	return c.JSON(toUserResponse(user, roleNames))
}

func (h *UserHandler) GetRoles(c *fiber.Ctx) error {
	roles, err := database.ListRoles(database.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(roles)
}
