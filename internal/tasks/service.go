package tasks

import (
	"errors"
	"strings"
	"time"

	"aigent/internal/database"

	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

type CreateTaskInput struct {
	Name           string `json:"name"`
	CronExpression string `json:"cron_expression"`
	AgentID        uint   `json:"agent_id"`
	Prompt         string `json:"prompt"`
	OneShot        bool   `json:"one_shot"`
}

func CreateScheduledTask(input CreateTaskInput) (*database.Task, error) {
	name := strings.TrimSpace(input.Name)
	cronExpression := strings.TrimSpace(input.CronExpression)
	prompt := strings.TrimSpace(input.Prompt)

	if name == "" {
		return nil, errors.New("name is required")
	}
	if cronExpression == "" {
		return nil, errors.New("cron_expression is required")
	}
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}

	if _, err := cronParser.Parse(cronExpression); err != nil {
		return nil, errors.New("invalid cron expression: " + err.Error())
	}

	agentID := input.AgentID
	if agentID == 0 {
		agentID = 1
	}

	var agent database.Agent
	if err := database.DB.First(&agent, agentID).Error; err != nil {
		return nil, errors.New("agent not found")
	}

	nextRun := CalculateNextRun(cronExpression, time.Now())
	task := &database.Task{
		Name:           name,
		CronExpression: cronExpression,
		AgentID:        agentID,
		Prompt:         prompt,
		OneShot:        input.OneShot,
		NextRunAt:      &nextRun,
	}

	if err := database.DB.Create(task).Error; err != nil {
		return nil, err
	}

	return task, nil
}

func CalculateNextRun(cronExpression string, now time.Time) time.Time {
	schedule, err := cronParser.Parse(cronExpression)
	if err != nil {
		return now.Add(24 * time.Hour)
	}
	return schedule.Next(now)
}
