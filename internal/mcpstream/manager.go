package mcpstream

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"

	"aigent/internal/database"
	"aigent/internal/utils"
)

// ServerEntry es una sesión activa con metadatos para SyncTools.
type ServerEntry struct {
	ID      uint
	Alias   string
	Session *Session
}

// Manager mantiene sesiones MCP streamable por ID de fila en BD.
type Manager struct {
	mu      sync.RWMutex
	entries map[uint]*ServerEntry
}

// NewManager crea un gestor vacío.
func NewManager() *Manager {
	return &Manager{
		entries: make(map[uint]*ServerEntry),
	}
}

// CloseAll cierra todas las sesiones (idempotente).
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.entries {
		if e != nil && e.Session != nil {
			if err := e.Session.Close(); err != nil {
				log.Printf("mcpstream: close server id=%d: %v", id, err)
			}
		}
	}
	m.entries = make(map[uint]*ServerEntry)
}

// ReloadFromDB cierra todo y vuelve a conectar servidores habilitados.
func (m *Manager) ReloadFromDB(ctx context.Context) {
	masterKey := os.Getenv("DB_ENCRYPTION_KEY")

	m.mu.Lock()
	for id, e := range m.entries {
		if e != nil && e.Session != nil {
			_ = e.Session.Close()
		}
		delete(m.entries, id)
	}

	var servers []database.McpStreamServer
	if err := database.DB.Where("enabled = ?", true).Order("id asc").Find(&servers).Error; err != nil {
		log.Printf("mcpstream: reload list servers: %v", err)
		m.mu.Unlock()
		return
	}

	for _, s := range servers {
		headers, err := DecryptHeadersCipher(s.HeadersCipher, masterKey)
		if err != nil {
			log.Printf("mcpstream [%s]: headers decrypt: %v", s.Alias, err)
			continue
		}

		sess, err := Connect(ctx, s.BaseURL, headers, s.DisableStandaloneSSE)
		if err != nil {
			log.Printf("mcpstream [%s]: connect failed: %v", s.Alias, err)
			continue
		}
		m.entries[s.ID] = &ServerEntry{
			ID:      s.ID,
			Alias:   s.Alias,
			Session: sess,
		}
		log.Printf("mcpstream: connected alias=%s id=%d", s.Alias, s.ID)
	}
	m.mu.Unlock()
}

// ListEntries devuelve una copia de las sesiones activas (para SyncTools).
func (m *Manager) ListEntries() []ServerEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ServerEntry, 0, len(m.entries))
	for _, e := range m.entries {
		if e == nil || e.Session == nil {
			continue
		}
		out = append(out, ServerEntry{
			ID:      e.ID,
			Alias:   e.Alias,
			Session: e.Session,
		})
	}
	return out
}

// TestConnection conecta, lista tools y cierra (para diagnóstico desde API).
func TestConnection(ctx context.Context, baseURL string, headers map[string]string, disableStandaloneSSE bool) (toolNames []string, err error) {
	sess, err := Connect(ctx, baseURL, headers, disableStandaloneSSE)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sess.Close() }()

	tools, err := sess.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tools {
		if t != nil {
			toolNames = append(toolNames, t.Name)
		}
	}
	return toolNames, nil
}

// EncryptHeadersCipher serializa y cifra el mapa de cabeceras HTTP (vacío → "").
func EncryptHeadersCipher(headers map[string]string, masterKey string) (string, error) {
	if len(headers) == 0 {
		return "", nil
	}
	b, err := json.Marshal(headers)
	if err != nil {
		return "", err
	}
	return utils.Encrypt(string(b), masterKey)
}

// DecryptHeadersCipher descifra a mapa (cadena vacía → nil map).
func DecryptHeadersCipher(cipherText, masterKey string) (map[string]string, error) {
	if cipherText == "" {
		return nil, nil
	}
	plain, err := utils.Decrypt(cipherText, masterKey)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(plain), &m); err != nil {
		return nil, err
	}
	return m, nil
}
