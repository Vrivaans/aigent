package ai

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"aigent/internal/database"
)

// ActiveSession representa una sesión de chat cargada en memoria activa
type ActiveSession struct {
	SessionID      uint
	Messages       []ChatMessage
	ContextSummary string
	LastAccessed   time.Time
	mu             sync.RWMutex
}

// SessionManager orquesta el caché de sesiones en memoria y su persistencia asíncrona
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[uint]*ActiveSession
}

var (
	globalSessionManager *SessionManager
	once                 sync.Once
)

// GetSessionManager obtiene la instancia singleton del SessionManager
func GetSessionManager() *SessionManager {
	once.Do(func() {
		globalSessionManager = &SessionManager{
			sessions: make(map[uint]*ActiveSession),
		}
		// Iniciar rutina de limpieza de sesiones inactivas cada 10 minutos
		go globalSessionManager.startCleanupLoop(10 * time.Minute)
	})
	return globalSessionManager
}

// GetOrCreateSession obtiene la sesión de la memoria o la carga desde la base de datos si no existe
func (sm *SessionManager) GetOrCreateSession(sessionID uint) (*ActiveSession, error) {
	sm.mu.Lock()
	session, exists := sm.sessions[sessionID]
	if exists {
		session.LastAccessed = time.Now()
		sm.mu.Unlock()
		return session, nil
	}

	// Caché miss: cargar de la base de datos
	session = &ActiveSession{
		SessionID:    sessionID,
		LastAccessed: time.Now(),
	}
	sm.sessions[sessionID] = session
	sm.mu.Unlock()

	// Cargar mensajes e información de la DB
	err := session.ReloadFromDB()
	if err != nil {
		sm.mu.Lock()
		delete(sm.sessions, sessionID)
		sm.mu.Unlock()
		return nil, err
	}

	return session, nil
}

// ClearSession elimina la sesión del caché de memoria
func (sm *SessionManager) ClearSession(sessionID uint) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
}

// startCleanupLoop elimina sesiones inactivas para liberar memoria
func (sm *SessionManager) startCleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for id, sess := range sm.sessions {
			sess.mu.RLock()
			// Si no ha sido accedida en las últimas 2 horas, la removemos de memoria
			if now.Sub(sess.LastAccessed) > 2*time.Hour {
				log.Printf("🧹 SessionManager: Evicting inactive session #%d from memory", id)
				delete(sm.sessions, id)
			}
			sess.mu.RUnlock()
		}
		sm.mu.Unlock()
	}
}

// ReloadFromDB carga toda la información de la sesión y los últimos 50 mensajes de chat
func (s *ActiveSession) ReloadFromDB() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var dbSession database.Session
	if err := database.DB.First(&dbSession, s.SessionID).Error; err != nil {
		return fmt.Errorf("failed to find session in DB: %w", err)
	}
	s.ContextSummary = dbSession.ContextSummary

	// Obtener los últimos 50 mensajes ordenados por created_at desc (más recientes primero)
	var recentDBMessages []database.ChatMessage
	err := database.DB.Where("session_id = ?", s.SessionID).
		Order("created_at desc").
		Limit(50).
		Find(&recentDBMessages).Error
	if err != nil {
		return fmt.Errorf("failed to load chat history from DB: %w", err)
	}

	// Invertir el array para reconstruir el orden cronológico ascendente (el orden requerido por el LLM)
	s.Messages = make([]ChatMessage, 0, len(recentDBMessages))

	for i := len(recentDBMessages) - 1; i >= 0; i-- {
		dbMsg := recentDBMessages[i]
		content := dbMsg.Content
		if content == "" {
			content = " " // Evitar contenido vacío
		}

		m := ChatMessage{
			Role:    dbMsg.Role,
			Content: content,
		}

		if dbMsg.Role == "tool" {
			m.ToolCallID = dbMsg.ToolCallID
		}

		if dbMsg.Role == "assistant" && dbMsg.RawToolCalls != "" {
			var tCalls []ToolCall
			if err := json.Unmarshal([]byte(dbMsg.RawToolCalls), &tCalls); err == nil {
				m.ToolCalls = tCalls
			}
		}
		s.Messages = append(s.Messages, m)
	}

	log.Printf("📥 SessionManager: Loaded session #%d with %d messages from DB", s.SessionID, len(s.Messages))
	return nil
}

// AddMessage agrega un mensaje al caché de memoria y dispara el guardado asíncrono en la DB
func (s *ActiveSession) AddMessage(role string, content string, toolCallID string, rawToolCalls string) ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Agregar a memoria
	if content == "" {
		content = " "
	}
	msg := ChatMessage{
		Role:       role,
		Content:    content,
		ToolCallID: toolCallID,
	}
	if rawToolCalls != "" {
		var tCalls []ToolCall
		if err := json.Unmarshal([]byte(rawToolCalls), &tCalls); err == nil {
			msg.ToolCalls = tCalls
		}
	}
	s.Messages = append(s.Messages, msg)
	s.LastAccessed = time.Now()

	// 2. Persistir asíncronamente en la base de datos
	dbMsg := database.ChatMessage{
		SessionID:    s.SessionID,
		Role:         role,
		Content:      content,
		ToolCallID:   toolCallID,
		RawToolCalls: rawToolCalls,
	}

	go func(m database.ChatMessage) {
		// SQLite/GORM maneja escrituras de manera concurrente. Si hay algún conflicto,
		// reintentamos brevemente. En un entorno de usuario único la contención es mínima.
		for retries := 0; retries < 3; retries++ {
			if err := database.DB.Create(&m).Error; err != nil {
				log.Printf("⚠️ SessionManager: DB write retry %d failed for message: %v", retries+1, err)
				time.Sleep(50 * time.Millisecond)
				continue
			}
			break
		}
	}(dbMsg)

	return msg
}

// GetMessages devuelve una copia segura de los mensajes activos para el LLM
func (s *ActiveSession) GetMessages() []ChatMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := make([]ChatMessage, len(s.Messages))
	copy(msgs, s.Messages)
	return msgs
}

// SaveContextSummary guarda un nuevo resumen consolidado en memoria y en la DB
func (s *ActiveSession) SaveContextSummary(summary string) {
	s.mu.Lock()
	s.ContextSummary = summary
	s.mu.Unlock()

	go func(sessionID uint, sum string) {
		err := database.DB.Model(&database.Session{}).
			Where("id = ?", sessionID).
			Update("context_summary", sum).Error
		if err != nil {
			log.Printf("⚠️ SessionManager: Failed to save context summary to DB for session #%d: %v", sessionID, err)
		}
	}(s.SessionID, summary)
}
