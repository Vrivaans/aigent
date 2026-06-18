package ai

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aigent/internal/database"
)

func buildRuntimeMessages(systemPrompt string, chatHistory []ChatMessage, newUserMsg string) []ChatMessage {
	messages := []ChatMessage{{Role: "system", Content: systemPrompt}}

	for _, msg := range chatHistory {
		content := msg.Content
		if content == "" {
			content = " "
		}
		m := ChatMessage{
			Role:       msg.Role,
			Content:    content,
			ToolCallID: msg.ToolCallID,
			ToolCalls:  msg.ToolCalls,
		}
		messages = append(messages, m)
	}

	if newUserMsg != "" {
		messages = append(messages, ChatMessage{Role: "user", Content: newUserMsg})
	}

	return messages
}

// scanWorkspace escanea el workspace y devuelve el contenido de todos los archivos concatenados de manera determinista.
func scanWorkspace(workspacePath string) (string, error) {
	if workspacePath == "" {
		return "", nil
	}

	cleanPath := filepath.Clean(workspacePath)
	info, err := os.Stat(cleanPath)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("invalid workspace directory: %w", err)
	}

	// Leer .aigentignore si existe en la raíz
	var ignorePatterns []string
	ignoreFilePath := filepath.Join(cleanPath, ".aigentignore")
	if bytes, err := os.ReadFile(ignoreFilePath); err == nil {
		lines := strings.Split(string(bytes), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				ignorePatterns = append(ignorePatterns, line)
			}
		}
	}

	// Exclusiones por defecto
	defaultIgnores := []string{
		".git", "node_modules", "vendor", ".gemini", "dist", "bin", "obj",
		".DS_Store", "tmp", "temp", "build", "server",
	}

	allowedExtensions := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true,
		".html": true, ".css": true, ".json": true, ".yaml": true, ".yml": true,
		".md": true, ".txt": true, ".ini": true, ".conf": true, ".sql": true,
		".sh": true, ".bat": true, ".proto": true, ".c": true, ".cpp": true,
		".h": true, ".java": true, ".cs": true, ".rs": true, ".kt": true,
		".swift": true, ".php": true, ".rb": true, ".pl": true,
	}

	var files []string
	err = filepath.WalkDir(cleanPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // ignorar directorios con errores de lectura individuales
		}

		relPath, relErr := filepath.Rel(cleanPath, path)
		if relErr != nil {
			return nil
		}

		// Chequear exclusiones por defecto
		for _, ignore := range defaultIgnores {
			if strings.Contains(path, "/"+ignore+"/") || strings.HasSuffix(path, "/"+ignore) || relPath == ignore || strings.HasPrefix(relPath, ignore+"/") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Chequear .aigentignore
		for _, pattern := range ignorePatterns {
			matched, _ := filepath.Match(pattern, relPath)
			if matched || strings.Contains(relPath, "/"+pattern+"/") || strings.HasPrefix(relPath, pattern+"/") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if d.IsDir() {
			return nil
		}

		// Filtrar por extensiones permitidas
		ext := strings.ToLower(filepath.Ext(path))
		if !allowedExtensions[ext] {
			return nil
		}

		// Limitar tamaño de archivo individual a 200KB
		if info, err := d.Info(); err == nil && info.Size() > 200*1024 {
			return nil
		}

		files = append(files, relPath)
		return nil
	})

	if err != nil {
		return "", err
	}

	// Orden determinista alfabético
	sort.Strings(files)

	var sb strings.Builder
	for _, relPath := range files {
		fullPath := filepath.Join(cleanPath, relPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n=== ARCHIVO DEL WORKSPACE: %s ===\n", relPath))
		sb.WriteString(string(content))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// buildLayer2Content assembles deterministic Layer 2 text (goals, session files, workspace).
// workspaceContent should be pre-scanned when WorkspacePath is set; pass "" to omit workspace block.
func buildLayer2Content(session database.Session, sessionFiles []database.SessionFile, workspaceContent string) string {
	var cap2SB strings.Builder

	if session.SessionGoals != "" {
		cap2SB.WriteString("=== OBJETIVOS Y REGLAS DE LA SESIÓN ===\n")
		cap2SB.WriteString(session.SessionGoals)
		cap2SB.WriteString("\n\n")
	}

	if len(sessionFiles) > 0 {
		cap2SB.WriteString("=== ARCHIVOS DE CONTEXTO DE LA SESIÓN ===\n")
		sort.Slice(sessionFiles, func(i, j int) bool {
			return sessionFiles[i].Filename < sessionFiles[j].Filename
		})
		for _, f := range sessionFiles {
			cap2SB.WriteString(fmt.Sprintf("--- ARCHIVO: %s ---\n", f.Filename))
			cap2SB.WriteString(f.Content)
			cap2SB.WriteString("\n\n")
		}
	}

	if workspaceContent != "" {
		cap2SB.WriteString("=== ARCHIVOS DEL REPOSITORIO DE CÓDIGO ===\n")
		cap2SB.WriteString(workspaceContent)
		cap2SB.WriteString("\n")
	}

	return cap2SB.String()
}

func buildLayer2ContentFromSession(session database.Session, sessionFiles []database.SessionFile) string {
	var workspaceContent string
	if session.WorkspacePath != "" {
		if wc, err := scanWorkspace(session.WorkspacePath); err == nil {
			workspaceContent = wc
		} else {
			log.Printf("⚠️ SCC: Error scanning workspace %s: %v", session.WorkspacePath, err)
		}
	}
	return buildLayer2Content(session, sessionFiles, workspaceContent)
}

// Layer2SHA256 returns the full SHA-256 hex digest of Layer 2 content (cache invalidation key).
func Layer2SHA256(session database.Session, sessionFiles []database.SessionFile, workspaceContent string) string {
	content := buildLayer2Content(session, sessionFiles, workspaceContent)
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}

func logLayer2Hash(cap2Content string, isAnthropic bool) {
	if cap2Content == "" {
		return
	}
	h := sha256.Sum256([]byte(cap2Content))
	log.Printf("🔒 SmartContextCache: Layer 2 SHA-256 Hash = %s | Anthropic Cache = %t", fmt.Sprintf("%x", h)[:8], isAnthropic)
}

// buildRuntimeMessagesWithCache compone los mensajes en tres capas jerárquicas y activa caching explícito si el proveedor es Anthropic.
func buildRuntimeMessagesWithCache(
	systemPrompt string,
	session database.Session,
	sessionFiles []database.SessionFile,
	chatHistory []ChatMessage,
	newUserMsg string,
	providerName string,
) []ChatMessage {
	var messages []ChatMessage

	// Capa 1: Núcleo del Sistema (System prompt base)
	// Contiene personalidad de AIgent,MCP Tools, y especificación de RuleGo.
	messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})

	cap2Content := buildLayer2ContentFromSession(session, sessionFiles)

	// Inyectar Capa 2 como mensaje de sistema separado si tiene contenido
	if cap2Content != "" {
		providerLower := strings.ToLower(providerName)
		isAnthropic := strings.Contains(providerLower, "anthropic") || strings.Contains(providerLower, "claude")

		var cc *CacheControl
		if isAnthropic {
			// Inyectar marca de control de caché efímero para Anthropic Claude
			cc = &CacheControl{Type: "ephemeral"}
		}

		messages = append(messages, ChatMessage{
			Role:         "system",
			Content:      "=== CONTEXTO DE SESIÓN Y ARCHIVOS DE CACHÉ ===\n" + cap2Content,
			CacheControl: cc,
		})

		logLayer2Hash(cap2Content, isAnthropic)
	}

	// Capa 3: Cola de Historial Dinámico e Interacción (Alta Volatilidad)
	for _, msg := range chatHistory {
		content := msg.Content
		if content == "" {
			content = " "
		}
		m := ChatMessage{
			Role:       msg.Role,
			Content:    content,
			ToolCallID: msg.ToolCallID,
			ToolCalls:  msg.ToolCalls,
		}
		messages = append(messages, m)
	}

	if newUserMsg != "" {
		messages = append(messages, ChatMessage{Role: "user", Content: newUserMsg})
	}

	return messages
}
