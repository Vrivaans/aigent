package database

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
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

type Session struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Title     string    `gorm:"size:255;not null" json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatMessage represents the conversation history between User and AIgent
type ChatMessage struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	SessionID    uint      `gorm:"not null;default:1" json:"session_id"`
	Role         string    `gorm:"size:50;not null" json:"role"` // e.g. "user", "assistant", "system", "tool"
	Content      string    `gorm:"type:text;not null" json:"content"`
	ToolCallID   string    `gorm:"size:100" json:"tool_call_id,omitempty"`
	RawToolCalls string    `gorm:"type:text" json:"raw_tool_calls,omitempty"` // JSON of []ToolCall
	CreatedAt    time.Time `json:"created_at"`
}

type PendingAction struct {
	gorm.Model
	SessionID  uint   `json:"session_id"`
	ToolName   string `json:"tool_name"`
	Arguments  string `json:"arguments"` // JSON representation
	ToolCallID string `json:"tool_call_id"`
	Status     string `json:"status"` // pending, approved, rejected
}

type LLMProvider struct {
	gorm.Model
	Name         string `json:"name" gorm:"unique"`
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`       // Encrypted
	DefaultModel string `json:"default_model"`
	IsActive     bool   `json:"is_active" gorm:"default:true"`
	IsDefault    bool   `json:"is_default" gorm:"default:false"`
}

// HandsAIConfig stores the connection settings for the real-world tool execution engine
type HandsAIConfig struct {
	gorm.Model
	Username string `json:"username" gorm:"uniqueIndex"`
	URL      string `json:"url"`
	Token    string `json:"token"` // Encrypted with AES-256 via DB_ENCRYPTION_KEY
}
