package database

import (
	"time"

	"github.com/pgvector/pgvector-go"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Tenant groups users and core configuration for multi-tenant isolation.
type Tenant struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Slug      string    `gorm:"size:64;uniqueIndex;not null" json:"slug"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents a login account with optional RBAC roles.
type User struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	TenantID     *uint     `gorm:"index" json:"tenant_id,omitempty"`
	Tenant       *Tenant   `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Username     string    `gorm:"size:255;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"type:text;not null" json:"-"`
	IsActive     bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Roles        []Role    `gorm:"many2many:user_roles;" json:"roles,omitempty"`
}

// Role groups permissions assigned to users.
type Role struct {
	ID          uint             `gorm:"primarykey" json:"id"`
	Name        string           `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description string           `gorm:"type:text" json:"description"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Permissions []RolePermission `gorm:"foreignKey:RoleID;constraint:OnDelete:CASCADE;" json:"permissions,omitempty"`
}

// RolePermission maps a role to a resource/action pair (e.g. agents + create).
type RolePermission struct {
	ID       uint   `gorm:"primarykey" json:"id"`
	RoleID   uint   `gorm:"not null;index;uniqueIndex:idx_role_resource_action" json:"role_id"`
	Resource string `gorm:"size:64;not null;uniqueIndex:idx_role_resource_action" json:"resource"`
	Action   string `gorm:"size:64;not null;uniqueIndex:idx_role_resource_action" json:"action"`
}

// UserRole links users to roles (many-to-many join table).
type UserRole struct {
	UserID uint `gorm:"primaryKey" json:"user_id"`
	RoleID uint `gorm:"primaryKey" json:"role_id"`
}

// AuditEvent is an append-only audit log row. Application code must not update or delete rows.
type AuditEvent struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	OccurredAt    time.Time `gorm:"not null;index" json:"occurred_at"`
	ActorUserID   *uint     `gorm:"index" json:"actor_user_id,omitempty"`
	Action        string    `gorm:"size:128;not null;index" json:"action"`
	ResourceType  string    `gorm:"size:64;not null;index:idx_audit_resource,priority:1" json:"resource_type"`
	ResourceID    string    `gorm:"size:64;not null;index:idx_audit_resource,priority:2" json:"resource_id"`
	SessionID     *uint     `gorm:"index" json:"session_id,omitempty"`
	IP            string    `gorm:"size:45" json:"ip,omitempty"`
	UserAgent     string    `gorm:"type:text" json:"user_agent,omitempty"`
	PayloadBefore *string   `gorm:"type:text" json:"payload_before,omitempty"`
	PayloadAfter  *string   `gorm:"type:text" json:"payload_after,omitempty"`
	CorrelationID string    `gorm:"size:64;index" json:"correlation_id,omitempty"`
}

// Agent represents a specialized AI persona with its own model and toolset
type Agent struct {
	ID            uint        `gorm:"primarykey" json:"id"`
	TenantID      *uint       `gorm:"index" json:"tenant_id,omitempty"`
	Tenant        *Tenant     `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Name          string      `gorm:"size:255;not null" json:"name"`
	Description   string      `gorm:"type:text" json:"description"`
	LLMProviderID *uint       `json:"llm_provider_id"`
	LLMProvider   LLMProvider `gorm:"foreignKey:LLMProviderID" json:"llm_provider"`
	Tools         []AgentTool `gorm:"foreignKey:AgentID;constraint:OnDelete:CASCADE;" json:"tools,omitempty"`
	ToolsCount    int         `gorm:"-" json:"tools_count,omitempty"`
	IsDefault     bool        `gorm:"default:false" json:"is_default"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// AgentTool links an agent to a specific HandsAI tool name
type AgentTool struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	AgentID   uint      `gorm:"not null" json:"agent_id"`
	ToolName  string    `gorm:"size:255;not null" json:"tool_name"`
	CreatedAt time.Time `json:"created_at"`
}

// Task represents a scheduled job that runs an agent flow with a prompt.
type Task struct {
	ID             uint       `gorm:"primarykey" json:"id"`
	Name           string     `gorm:"size:255;not null" json:"name"`
	CronExpression string     `gorm:"size:100;not null" json:"cron_expression"`
	AgentID        uint       `gorm:"not null;default:1" json:"agent_id"`
	Agent          Agent      `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
	Prompt         string     `gorm:"type:text;not null" json:"prompt"`
	OneShot        bool       `gorm:"default:false" json:"one_shot"`
	NextRunAt      *time.Time `json:"next_run_at"`
	LastRunAt      *time.Time `json:"last_run_at"`
	LastResult     string     `gorm:"type:text" json:"last_result,omitempty"`
	LastError      string     `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ToolPermission stores persistent allow/deny decisions per agent+tool combination.
type ToolPermission struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	AgentID    uint       `gorm:"not null;index" json:"agent_id"`
	ToolName   string     `gorm:"size:255;not null;index" json:"tool_name"`
	ActionType string     `gorm:"size:20;not null;default:'always_allow'" json:"action_type"` // always_allow | always_deny
	Paused     bool       `gorm:"default:false" json:"paused"`
	PausedAt   *time.Time `json:"paused_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Rule represents behavioral constraints or configuration injected into OpenRouter Prompts
type Rule struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	Agents     []Agent   `gorm:"many2many:rule_agents;" json:"agents"`
	Category   string    `gorm:"size:100;not null" json:"category"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	Importance int       `gorm:"default:1" json:"importance"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Session struct {
	ID                    uint         `gorm:"primarykey" json:"id"`
	TenantID              *uint        `gorm:"index" json:"tenant_id,omitempty"`
	Tenant                *Tenant      `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Title                 string       `gorm:"size:255;not null" json:"title"`
	AgentID               uint         `gorm:"not null;default:1" json:"agent_id"`
	Agent                 *Agent       `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
	TaskID                *uint        `json:"task_id,omitempty"`
	LLMProviderOverrideID *uint        `json:"llm_provider_override_id,omitempty"`
	LLMProviderOverride   *LLMProvider `gorm:"foreignKey:LLMProviderOverrideID" json:"llm_provider_override,omitempty"`
	LLMModelOverride      string       `gorm:"size:255" json:"llm_model_override,omitempty"`
	ContextSummary        string       `gorm:"type:text" json:"context_summary,omitempty"`
	SessionGoals          string       `gorm:"type:text" json:"session_goals,omitempty"`
	WorkspacePath         string       `gorm:"size:512" json:"workspace_path,omitempty"`
	Layer2Hash            string       `gorm:"size:64;index" json:"layer2_hash,omitempty"`
	ProviderCacheID       string       `gorm:"size:512" json:"provider_cache_id,omitempty"`
	CacheExpiresAt        *time.Time   `json:"cache_expires_at,omitempty"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             time.Time    `json:"updated_at"`
}

// SessionFile represents a file ingested to be cached as session context (Layer 2)
type SessionFile struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	SessionID uint      `gorm:"not null;index" json:"session_id"`
	Filename  string    `gorm:"size:255;not null" json:"filename"`
	Content   string    `gorm:"type:text;not null" json:"content"` // Markdown/Plain Text content
	Hash      string    `gorm:"size:64;not null" json:"hash"`      // SHA-256 hash of the content
	CreatedAt time.Time `json:"created_at"`
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

// Artifact represents code, diagrams, HTML or markdown mutable.
type Artifact struct {
	ID        string    `gorm:"primaryKey;size:64" json:"id"`
	SessionID uint      `gorm:"not null;index" json:"session_id"`
	Type      string    `gorm:"size:30" json:"type"` // e.g. "diagram"
	Title     string    `gorm:"size:255" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Notification: mensaje de un agente al usuario, linkeado a la sesión que lo
// originó para continuar el contexto desde el centro de notificaciones.
type Notification struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	SessionID uint       `gorm:"index" json:"session_id"`
	Title     string     `gorm:"size:255;not null" json:"title"`
	Body      string     `gorm:"type:text" json:"body"`
	Level     string     `gorm:"size:20;not null;default:info" json:"level"` // info|success|warning
	ReadAt    *time.Time `gorm:"index" json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type PendingAction struct {
	ID               uint           `gorm:"primarykey" json:"id"`
	SessionID        uint           `json:"session_id"`
	ToolName         string         `json:"tool_name"`
	Arguments        string         `json:"arguments"` // JSON representation
	ToolCallID       string         `json:"tool_call_id"`
	Status           string         `json:"status"` // PENDING, APPROVED, REJECTED
	ResolvedByUserID *uint          `gorm:"index" json:"resolved_by_user_id,omitempty"`
	ResolvedAt       *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// ApprovalPolicy defines when a tool pattern requires human approval beyond registry defaults.
type ApprovalPolicy struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	ToolPattern       string    `gorm:"size:255;not null;index" json:"tool_pattern"`
	Environment       string    `gorm:"size:64;not null;default:*" json:"environment"`
	RequiresApproval  bool      `gorm:"not null;default:true" json:"requires_approval"`
	MinRole           string    `gorm:"size:64;not null;default:operator" json:"min_role"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type LLMProvider struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	TenantID     *uint          `gorm:"index" json:"tenant_id,omitempty"`
	Tenant       *Tenant        `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Name         string         `json:"name" gorm:"unique"`
	BaseURL      string         `json:"base_url"`
	APIKey       string         `json:"api_key"` // Encrypted
	DefaultModel string         `json:"default_model"`
	ProviderType string         `gorm:"size:50;default:'custom'" json:"provider_type"` // zen, groq, openrouter, openai, custom
	IsActive     bool           `json:"is_active" gorm:"default:true"`
	IsDefault    bool           `json:"is_default" gorm:"default:false"`
	IsEmbeddings bool           `json:"is_embeddings" gorm:"default:false"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// Model represents an individual LLM available from a provider.
type Model struct {
	ID            uint         `gorm:"primarykey" json:"id"`
	ProviderID    uint         `gorm:"not null;index" json:"provider_id"`
	Provider      LLMProvider  `gorm:"foreignKey:ProviderID" json:"provider,omitempty"`
	ModelID       string       `gorm:"size:255;not null" json:"model_id"`
	Name          string       `gorm:"size:255;not null" json:"name"`
	IsFree        bool         `gorm:"default:false" json:"is_free"`
	ContextWindow int          `gorm:"default:0" json:"context_window"`
	LastSeen      time.Time    `json:"last_seen"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

func ProviderPresetBaseURL(providerType string) string {
	switch providerType {
	case "zen":
		return "https://opencode.ai/zen/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "openai":
		return "https://api.openai.com/v1"
	default:
		return ""
	}
}

// HandsAIConfig stores the connection settings for the real-world tool execution engine
type HandsAIConfig struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	TenantID  *uint          `gorm:"index" json:"tenant_id,omitempty"`
	Tenant    *Tenant        `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Username  string         `json:"username" gorm:"uniqueIndex"`
	URL       string         `json:"url"`
	Token     string         `json:"token"` // Encrypted with AES-256 via DB_ENCRYPTION_KEY
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// McpStdioServer define un proceso MCP local (stdio) arrancado con command + args.
type McpStdioServer struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Alias     string         `gorm:"size:64;not null;uniqueIndex" json:"alias"`
	Command   string         `gorm:"size:512;not null" json:"command"`
	ArgsJSON  datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"-"`
	EnvCipher string         `gorm:"type:text" json:"-"` // JSON map cifrado (DB_ENCRYPTION_KEY)
	Enabled   bool           `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// McpStreamServer define un servidor MCP remoto (HTTP streamable / SSE).
type McpStreamServer struct {
	ID                   uint           `gorm:"primarykey" json:"id"`
	Alias                string         `gorm:"size:64;not null;uniqueIndex" json:"alias"`
	BaseURL              string         `gorm:"size:2048;not null" json:"base_url"`
	HeadersCipher        string         `gorm:"type:text" json:"-"` // JSON map cifrado (Authorization, etc.)
	DisableStandaloneSSE bool           `gorm:"default:false" json:"disable_standalone_sse"`
	Enabled              bool           `gorm:"default:true" json:"enabled"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

type Workflow struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	Name           string    `gorm:"size:255;not null" json:"name"`
	Description    string    `gorm:"type:text" json:"description"`
	CronExpression string    `gorm:"size:100" json:"cron_expression,omitempty"`
	Definition     string    `gorm:"type:text;not null" json:"definition"` // JSON de RuleChain
	Enabled        bool      `gorm:"default:true" json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type WorkflowRun struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	WorkflowID    uint      `gorm:"not null;index" json:"workflow_id"`
	Status        string    `gorm:"size:50;default:'RUNNING'" json:"status"` // RUNNING, COMPLETED, FAILED, PAUSED, WAITING
	CurrentNodeID string    `gorm:"size:100" json:"current_node_id,omitempty"`
	Logs          string    `gorm:"type:text" json:"logs,omitempty"`
	// Durable execution
	InputPayload  string `gorm:"type:text" json:"input_payload,omitempty"`   // Payload original para replay
	OutputPayload string `gorm:"type:text" json:"output_payload,omitempty"`  // Último mensaje de salida
	ContextJSON   string `gorm:"type:text" json:"context_json,omitempty"`    // Blackboard KV del run (JSON)
	ParentRunID   *uint  `gorm:"index" json:"parent_run_id,omitempty"`       // Run padre si es subflujo
	WaitReason    string `gorm:"size:255" json:"wait_reason,omitempty"`      // ej. "agent_task:12"
	MissionID     *uint  `gorm:"index" json:"mission_id,omitempty"`          // Misión a la que pertenece
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// WorkflowCheckpoint guarda la salida de un nodo ya ejecutado dentro de un run.
// Permite replay idempotente: al reanudar, los nodos con checkpoint no se re-ejecutan.
type WorkflowCheckpoint struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	RunID        uint      `gorm:"not null;uniqueIndex:idx_run_node" json:"run_id"`
	NodeID       string    `gorm:"size:100;not null;uniqueIndex:idx_run_node" json:"node_id"`
	RelationType string    `gorm:"size:50" json:"relation_type"`        // Success | Failure
	MsgData      string    `gorm:"type:text" json:"msg_data,omitempty"` // Salida del nodo
	MetadataJSON string    `gorm:"type:text" json:"metadata_json,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// AgentTask representa la ejecución durable de un sub-agente (nodo aigent/agent).
// Sobrevive reinicios: el durable worker re-conduce tareas RUNNING/WAITING_APPROVAL.
type AgentTask struct {
	ID          uint       `gorm:"primarykey" json:"id"`
	SessionID   uint       `gorm:"not null;index" json:"session_id"`
	AgentID     uint       `gorm:"not null" json:"agent_id"`
	ParentRunID *uint      `gorm:"index" json:"parent_run_id,omitempty"` // WorkflowRun que la espera
	NodeID      string     `gorm:"size:100" json:"node_id,omitempty"`    // Nodo origen en el run padre
	MissionID   *uint      `gorm:"index" json:"mission_id,omitempty"`
	Status      string     `gorm:"size:50;default:'RUNNING'" json:"status"` // RUNNING, COMPLETED, FAILED, WAITING_APPROVAL
	Prompt      string     `gorm:"type:text" json:"prompt"`
	AutoApprove bool       `gorm:"default:false" json:"auto_approve"` // Pre-aprobar tools sensibles en esta task
	Output      string     `gorm:"type:text" json:"output,omitempty"`
	Error       string     `gorm:"type:text" json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Mission es una unidad de trabajo de larga duración (ej. "video sobre hackeo X").
// Agrupa runs, agent tasks y artifacts alrededor de un objetivo con estado compartido.
type Mission struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Title         string    `gorm:"size:255;not null" json:"title"`
	Topic         string    `gorm:"type:text" json:"topic,omitempty"`
	Goal          string    `gorm:"type:text" json:"goal,omitempty"`
	Status        string    `gorm:"size:50;default:'ACTIVE'" json:"status"` // ACTIVE, COMPLETED, FAILED, PAUSED
	ContextJSON   string    `gorm:"type:text" json:"context_json,omitempty"` // Blackboard KV compartido (JSON)
	WorkspacePath string    `gorm:"size:512" json:"workspace_path,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// MissionArtifact es un entregable producido dentro de una misión
// (research.md, screenshots, guion, etc.)
type MissionArtifact struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	MissionID uint      `gorm:"not null;index" json:"mission_id"`
	Name      string    `gorm:"size:255;not null" json:"name"` // ej. "research.md"
	Type      string    `gorm:"size:50" json:"type"`           // markdown, json, image, script...
	Content   string    `gorm:"type:text" json:"content,omitempty"`
	Path      string    `gorm:"size:512" json:"path,omitempty"` // Ruta en disco si es archivo grande
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DocumentChunk represents a chunked document segment with its embedding vector
type DocumentChunk struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	Source    string          `gorm:"size:255;not null;index" json:"source"`
	Content   string          `gorm:"type:text;not null" json:"content"`
	Embedding pgvector.Vector `gorm:"type:vector" json:"-"`
	CreatedAt time.Time       `json:"created_at"`
}
