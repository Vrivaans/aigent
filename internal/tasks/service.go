package tasks

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"aigent/internal/database"

	"gorm.io/datatypes"
)

type CreateTaskInput struct {
	Name           string          `json:"name"`
	CronExpression string          `json:"cron_expression"`
	ToolName       string          `json:"tool_name"`
	Payload        json.RawMessage `json:"payload"`
}

func CreateScheduledTask(input CreateTaskInput) (*database.Task, error) {
	name := strings.TrimSpace(input.Name)
	cronExpression := strings.TrimSpace(input.CronExpression)
	toolName := strings.TrimSpace(input.ToolName)

	if name == "" {
		return nil, errors.New("name is required")
	}
	if cronExpression == "" {
		return nil, errors.New("cron_expression is required")
	}
	if toolName == "" {
		return nil, errors.New("tool_name is required")
	}

	payload := input.Payload
	if len(payload) == 0 || string(payload) == "null" {
		payload = json.RawMessage(`{}`)
	}

	var payloadObject map[string]interface{}
	if err := json.Unmarshal(payload, &payloadObject); err != nil {
		return nil, errors.New("payload must be a valid JSON object")
	}

	nextRun := CalculateNextRun(cronExpression, time.Now())
	task := &database.Task{
		Name:           name,
		CronExpression: cronExpression,
		ToolName:       toolName,
		Payload:        datatypes.JSON(payload),
		NextRunAt:      &nextRun,
	}

	if err := database.DB.Create(task).Error; err != nil {
		return nil, err
	}

	return task, nil
}

func CalculateNextRun(cronExpression string, now time.Time) time.Time {
	cronLower := strings.ToLower(strings.TrimSpace(cronExpression))
	if strings.Contains(cronLower, "hourly") || strings.Contains(cronLower, "@hourly") {
		return now.Add(time.Hour)
	}
	if strings.Contains(cronLower, "* * * * *") {
		return now.Add(time.Minute)
	}
	return now.Add(24 * time.Hour)
}
