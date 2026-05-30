package handlers

import (
	"aigent/internal/database"
	tasksvc "aigent/internal/tasks"

	"github.com/gofiber/fiber/v2"
)

type TaskHandler struct{}

func (h *TaskHandler) GetTasks(c *fiber.Ctx) error {
	var tasks []database.Task
	if err := database.DB.Preload("Agent").Order("created_at desc").Find(&tasks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tasks)
}

func (h *TaskHandler) CreateTask(c *fiber.Ctx) error {
	var input tasksvc.CreateTaskInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON body"})
	}

	task, err := tasksvc.CreateScheduledTask(input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(task)
}

func (h *TaskHandler) DeleteTask(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Delete(&database.Task{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "deleted"})
}
