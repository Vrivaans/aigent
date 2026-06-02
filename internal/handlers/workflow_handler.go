package handlers

import (
	"strconv"

	"aigent/internal/ai"
	"aigent/internal/database"
	"aigent/internal/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type WorkflowHandler struct{}

type WorkflowResponse struct {
	database.Workflow
	Mermaid string `json:"mermaid"`
}

// GetWorkflows devuelve la lista de workflows
func (h *WorkflowHandler) GetWorkflows(c *fiber.Ctx) error {
	var list []database.Workflow
	if err := database.DB.Order("created_at desc").Find(&list).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(list)
}

// GetWorkflow devuelve un workflow y su correspondiente diagrama Mermaid
func (h *WorkflowHandler) GetWorkflow(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid workflow ID"})
	}

	var wf database.Workflow
	if err := database.DB.First(&wf, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Workflow not found"})
	}

	// Traducir definición de RuleGo a Mermaid
	mermaidStr, err := utils.RuleGoToMermaid(wf.Definition, "")
	if err != nil {
		mermaidStr = "graph TD\n  err[\"Error generando diagrama: " + err.Error() + "\"]"
	}

	return c.JSON(WorkflowResponse{
		Workflow: wf,
		Mermaid:  mermaidStr,
	})
}

type CreateWorkflowInput struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	CronExpression string `json:"cron_expression,omitempty"`
	Definition     string `json:"definition"` // JSON string de RuleChain
}

// CreateWorkflow crea o actualiza un workflow
func (h *WorkflowHandler) CreateWorkflow(c *fiber.Ctx) error {
	var input CreateWorkflowInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Normalizar y validar JSON de la definición
	normalizedDef, err := utils.NormalizeRuleChainJSON(input.Definition)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid workflow definition: " + err.Error()})
	}

	wf := database.Workflow{
		Name:           input.Name,
		Description:    input.Description,
		CronExpression: input.CronExpression,
		Definition:     normalizedDef,
		Enabled:        true,
	}

	if err := database.DB.Create(&wf).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Cargar en el motor de RuleGo en caliente
	_ = ai.ReloadWorkflows()

	return c.JSON(wf)
}

// ReloadWorkflows fuerza la recarga de todos los flujos activos de la base de datos en el motor RuleGo
func (h *WorkflowHandler) ReloadWorkflows(c *fiber.Ctx) error {
	if err := ai.ReloadWorkflows(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "reloaded"})
}

// RunWorkflow inicia una ejecución manual del workflow en segundo plano
func (h *WorkflowHandler) RunWorkflow(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var input struct {
		Payload string `json:"payload"`
	}
	_ = c.BodyParser(&input)
	if input.Payload == "" {
		input.Payload = "{}"
	}

	runID, err := ai.TriggerWorkflow(c.Context(), uint(id), input.Payload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "triggered",
		"run_id": runID,
	})
}

// GetWorkflowRuns devuelve el historial de ejecuciones de un workflow
func (h *WorkflowHandler) GetWorkflowRuns(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var runs []database.WorkflowRun
	if err := database.DB.Where("workflow_id = ?", id).Order("created_at desc").Find(&runs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(runs)
}

type WorkflowRunResponse struct {
	database.WorkflowRun
	Mermaid string `json:"mermaid"`
}

// GetWorkflowRun devuelve detalles de una ejecución específica y resalta el nodo activo en Mermaid
func (h *WorkflowHandler) GetWorkflowRun(c *fiber.Ctx) error {
	runIDStr := c.Params("run_id")
	runID, err := strconv.Atoi(runIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid Run ID"})
	}

	var run database.WorkflowRun
	if err := database.DB.First(&run, runID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Run not found"})
	}

	var wf database.Workflow
	if err := database.DB.First(&wf, run.WorkflowID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Workflow associated not found"})
	}

	// Resaltar el nodo activo (CurrentNodeID) en el diagrama Mermaid de la ejecución
	mermaidStr, err := utils.RuleGoToMermaid(wf.Definition, run.CurrentNodeID)
	if err != nil {
		mermaidStr = "graph TD\n  err[\"Error generando diagrama: " + err.Error() + "\"]"
	}

	return c.JSON(WorkflowRunResponse{
		WorkflowRun: run,
		Mermaid:     mermaidStr,
	})
}

// DeleteWorkflow elimina un workflow y su historial
func (h *WorkflowHandler) DeleteWorkflow(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// Iniciar transacción de base de datos
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Borrar ejecuciones
		if err := tx.Where("workflow_id = ?", id).Delete(&database.WorkflowRun{}).Error; err != nil {
			return err
		}
		// 2. Borrar workflow
		if err := tx.Delete(&database.Workflow{}, id).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete workflow: " + err.Error()})
	}

	return c.JSON(fiber.Map{"status": "deleted"})
}
