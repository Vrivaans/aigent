package database

import (
	"time"

	"gorm.io/datatypes"
)

// Task represents a scheduled job to be executed dynamically using HandsAI Tools
type Task struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	Name           string         `gorm:"size:255;not null" json:"name"`
	CronExpression string         `gorm:"size:100;not null" json:"cron_expression"`
	ToolName       string         `gorm:"size:255;not null" json:"tool_name"`
	Payload        datatypes.JSON `json:"payload"`
	NextRunAt      *time.Time     `json:"next_run_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// Rule represents behavioral constraints or configuration injected into OpenRouter Prompts
type Rule struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	Category   string    `gorm:"size:100;not null" json:"category"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	Importance int       `gorm:"default:1" json:"importance"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ChatMessage represents the conversation history between User and AIgent
type ChatMessage struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Role      string    `gorm:"size:50;not null" json:"role"` // e.g. "user", "assistant", "system"
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
