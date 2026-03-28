package database

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Agent represents a specialized AI persona with its own model and toolset
type Agent struct {
	ID            uint         `gorm:"primarykey" json:"id"`
	Name          string       `gorm:"size:255;not null" json:"name"`
	Description   string       `gorm:"type:text" json:"description"`
	LLMProviderID *uint        `json:"llm_provider_id"`
	LLMProvider   LLMProvider  `gorm:"foreignKey:LLMProviderID" json:"llm_provider"`
	Tools         []AgentTool  `gorm:"foreignKey:AgentID;constraint:OnDelete:CASCADE;" json:"tools"`
	IsDefault     bool         `gorm:"default:false" json:"is_default"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// AgentTool links an agent to a specific HandsAI tool name
type AgentTool struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	AgentID   uint      `gorm:"not null" json:"agent_id"`
	ToolName  string    `gorm:"size:255;not null" json:"tool_name"`
	CreatedAt time.Time `json:"created_at"`
}

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
	AgentID    *uint     `json:"agent_id"`
	Agent      *Agent    `gorm:"foreignKey:AgentID;constraint:OnDelete:CASCADE;" json:"agent,omitempty"`
	Category   string    `gorm:"size:100;not null" json:"category"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	Importance int       `gorm:"default:1" json:"importance"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Session struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Title     string    `gorm:"size:255;not null" json:"title"`
	AgentID   uint      `gorm:"not null;default:1" json:"agent_id"`
	Agent     *Agent    `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
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
	ID         uint           `gorm:"primarykey" json:"id"`
	SessionID  uint           `json:"session_id"`
	ToolName   string         `json:"tool_name"`
	Arguments  string         `json:"arguments"` // JSON representation
	ToolCallID string         `json:"tool_call_id"`
	Status     string         `json:"status"` // pending, approved, rejected
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type LLMProvider struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	Name         string         `json:"name" gorm:"unique"`
	BaseURL      string         `json:"base_url"`
	APIKey       string         `json:"api_key"`       // Encrypted
	DefaultModel string         `json:"default_model"`
	IsActive     bool           `json:"is_active" gorm:"default:true"`
	IsDefault    bool           `json:"is_default" gorm:"default:false"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// HandsAIConfig stores the connection settings for the real-world tool execution engine
type HandsAIConfig struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Username  string         `json:"username" gorm:"uniqueIndex"`
	URL       string         `json:"url"`
	Token     string         `json:"token"` // Encrypted with AES-256 via DB_ENCRYPTION_KEY
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
