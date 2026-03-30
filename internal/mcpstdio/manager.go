package mcpstdio

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

// Manager mantiene sesiones stdio MCP por ID de fila en BD.
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
				log.Printf("mcpstdio: close server id=%d: %v", id, err)
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

	var servers []database.McpStdioServer
	if err := database.DB.Where("enabled = ?", true).Order("id asc").Find(&servers).Error; err != nil {
		log.Printf("mcpstdio: reload list servers: %v", err)
		m.mu.Unlock()
		return
	}

	for _, s := range servers {
		args, err := ParseArgsJSON(s.ArgsJSON)
		if err != nil {
			log.Printf("mcpstdio [%s]: bad args json: %v", s.Alias, err)
			continue
		}
		env, err := DecryptEnvCipher(s.EnvCipher, masterKey)
		if err != nil {
			log.Printf("mcpstdio [%s]: env decrypt: %v", s.Alias, err)
			continue
		}

		sess, err := Connect(ctx, s.Command, args, env)
		if err != nil {
			log.Printf("mcpstdio [%s]: connect failed: %v", s.Alias, err)
			continue
		}
		m.entries[s.ID] = &ServerEntry{
			ID:      s.ID,
			Alias:   s.Alias,
			Session: sess,
		}
		log.Printf("mcpstdio: connected alias=%s id=%d", s.Alias, s.ID)
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
func TestConnection(ctx context.Context, command string, args []string, env map[string]string) (toolNames []string, err error) {
	sess, err := Connect(ctx, command, args, env)
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

// ParseArgsJSON decodifica ArgsJSON de BD.
func ParseArgsJSON(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var args []string
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	return args, nil
}

// EncryptEnvCipher serializa y cifra el mapa de entorno (vacío → "").
func EncryptEnvCipher(env map[string]string, masterKey string) (string, error) {
	if len(env) == 0 {
		return "", nil
	}
	b, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return utils.Encrypt(string(b), masterKey)
}

// DecryptEnvCipher descifra a mapa (cadena vacía → nil map).
func DecryptEnvCipher(cipherText, masterKey string) (map[string]string, error) {
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
