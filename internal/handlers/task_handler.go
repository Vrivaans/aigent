package handlers

import (
	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
)

type TaskHandler struct{}

// GetTasks lista las tareas background persistidas
func (h *TaskHandler) GetTasks(c *fiber.Ctx) error {
	var tasks []database.Task
	if err := database.DB.Order("created_at desc").Find(&tasks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tasks)
}

// DeleteTask elimina una tarea agendada remotamente 
func (h *TaskHandler) DeleteTask(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Delete(&database.Task{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "deleted"})
}
