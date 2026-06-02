package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type SkillMetadata struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Script      string          `json:"script"`
	Sensitive   bool            `json:"sensitive"`
}

// LoadSkills scanea el directorio de habilidades y las registra en el ToolRegistry
func LoadSkills(skillsDir string, registry *ToolRegistry) error {
	// Asegurar que el directorio de skills existe
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Errorf("failed to create skills directory: %w", err)
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

	loadedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subDirName := entry.Name()
		subDirPath := filepath.Join(skillsDir, subDirName)

		metadataPath := filepath.Join(subDirPath, "metadata.json")
		if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
			continue
		}

		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			log.Printf("⚠️ [Skills] Failed to read metadata.json in %s: %v", subDirName, err)
			continue
		}

		var metadata SkillMetadata
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			log.Printf("⚠️ [Skills] Failed to parse metadata.json in %s: %v", subDirName, err)
			continue
		}

		if metadata.Name == "" || metadata.Script == "" {
			log.Printf("⚠️ [Skills] Invalid metadata in %s (missing name or script)", subDirName)
			continue
		}

		scriptPath := filepath.Join(subDirPath, metadata.Script)
		metadataName := metadata.Name
		metadataSensitive := metadata.Sensitive

		// Registrar la habilidad en el ToolRegistry
		registry.Register(ToolDef{
			Name:        metadataName,
			Description: metadata.Description,
			Parameters:  metadata.Parameters,
			Sensitive:   metadataSensitive,
			Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
				// Validar existencia del script al momento de la ejecución
				if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
					return nil, fmt.Errorf("script file not found: %s", scriptPath)
				}

				argsBytes, err := json.Marshal(args)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal skill arguments: %w", err)
				}

				// Ejecutar Python3
				cmd := exec.CommandContext(ctx, "python3", scriptPath)
				var stdinBuf bytes.Buffer
				stdinBuf.Write(argsBytes)
				cmd.Stdin = &stdinBuf

				var stdoutBuf, stderrBuf bytes.Buffer
				cmd.Stdout = &stdoutBuf
				cmd.Stderr = &stderrBuf

				if err := cmd.Run(); err != nil {
					log.Printf("❌ [Skills: %s] Execution failed: %v. Stderr: %s", metadataName, err, stderrBuf.String())
					return nil, fmt.Errorf("skill execution failed: %w (stderr: %s)", err, stderrBuf.String())
				}

				return json.RawMessage(stdoutBuf.Bytes()), nil
			},
		})
		loadedCount++
	}

	if loadedCount > 0 {
		log.Printf("🔌 [Skills] Loaded %d dynamic Python skills from %s", loadedCount, skillsDir)
	}
	return nil
}
