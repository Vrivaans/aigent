package handlers

import (
	"encoding/json"
	"strings"

	"aigent/internal/ai"
	"aigent/internal/database"
	tasksvc "aigent/internal/tasks"

	"github.com/gofiber/fiber/v2"
)

type TaskHandler struct {
	Brain *ai.Brain
}

// GetTasks lista las tareas background persistidas
func (h *TaskHandler) GetTasks(c *fiber.Ctx) error {
	var tasks []database.Task
	if err := database.DB.Order("created_at desc").Find(&tasks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tasks)
}

func (h *TaskHandler) CreateTask(c *fiber.Ctx) error {
	var input tasksvc.CreateTaskInput
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}

	toolName := strings.TrimSpace(input.ToolName)
	if h.Brain != nil && toolName != "" {
		if _, exists := h.Brain.Registry.Get(input.ToolName); !exists {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tool_name does not exist in the active registry"})
		}
	}

	task, err := tasksvc.CreateScheduledTask(input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(task)
}

// DeleteTask elimina una tarea agendada remotamente
func (h *TaskHandler) DeleteTask(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Delete(&database.Task{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "deleted"})
}
