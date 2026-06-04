package handlers

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"aigent/internal/ai"
	"aigent/internal/database"
	"aigent/internal/rag"
	"aigent/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

// UploadKnowledgeFile handles POST /api/rag/upload
func UploadKnowledgeFile(c *fiber.Ctx) error {
	// 1. Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "no file uploaded"})
	}

	// Read optional configuration parameters
	chunkSizeStr := c.FormValue("chunk_size", "500")
	chunkOverlapStr := c.FormValue("chunk_overlap", "50")
	embeddingModel := c.FormValue("embedding_model", "text-embedding-3-small")

	chunkSize, _ := strconv.Atoi(chunkSizeStr)
	chunkOverlap, _ := strconv.Atoi(chunkOverlapStr)

	// 2. Open file stream
	fileStream, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to open uploaded file"})
	}
	defer fileStream.Close()

	// 3. Parse and chunk file using LangChaingo
	docs, err := rag.ParseAndSplitDocument(c.Context(), fileStream, file.Filename, chunkSize, chunkOverlap)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("failed to parse document: %v", err)})
	}

	if len(docs) == 0 {
		return c.JSON(fiber.Map{
			"status":  "success",
			"message": "file was processed but produced 0 chunks",
			"chunks":  0,
		})
	}

	// 4. Load a suitable provider to generate embeddings
	var provider database.LLMProvider
	providerIDStr := c.FormValue("provider_id")
	if providerIDStr != "" {
		pID, err := strconv.Atoi(providerIDStr)
		if err == nil {
			database.DB.Where("id = ? AND is_active = ?", pID, true).First(&provider)
		}
	}

	if provider.ID == 0 {
		database.DB.Where("is_active = ? AND is_embeddings = ?", true, true).First(&provider)
	}

	if provider.ID == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "No embeddings provider is configured. Please designate at least one active LLM provider for embeddings."})
	}

	// Determine embedding model: use provider's DefaultModel if non-empty, otherwise fallback
	if embeddingModel == "text-embedding-3-small" || embeddingModel == "" {
		if provider.DefaultModel != "" {
			embeddingModel = provider.DefaultModel
		} else {
			embeddingModel = "text-embedding-3-small"
			provName := strings.ToLower(provider.Name)
			provURL := strings.ToLower(provider.BaseURL)
			if strings.Contains(provName, "gemini") || strings.Contains(provURL, "gemini") {
				embeddingModel = "text-embedding-004"
			}
		}
	}

	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	apiKey, err := utils.Decrypt(provider.APIKey, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to decrypt provider API key"})
	}

	llmClient := ai.NewClient(apiKey, provider.BaseURL)

	// 5. Generate embeddings and save chunks within a transaction
	tx := database.DB.Begin()
	for _, doc := range docs {
		content := strings.TrimSpace(doc.PageContent)
		if content == "" {
			continue
		}

		vector, err := llmClient.CreateEmbeddings(c.Context(), content, embeddingModel)
		if err != nil {
			tx.Rollback()
			return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("failed to generate embedding: %v", err)})
		}

		chunk := database.DocumentChunk{
			Source:    file.Filename,
			Content:   content,
			Embedding: pgvector.NewVector(vector),
			CreatedAt: time.Now(),
		}

		if err := tx.Create(&chunk).Error; err != nil {
			tx.Rollback()
			return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("failed to save chunk: %v", err)})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to commit transaction"})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": fmt.Sprintf("successfully uploaded and indexed file %s", file.Filename),
		"chunks":  len(docs),
	})
}

// SearchKnowledge handles GET /api/rag/search
func SearchKnowledge(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		query = c.Query("query")
	}
	if query == "" {
		return c.Status(400).JSON(fiber.Map{"error": "query parameter 'q' or 'query' is required"})
	}

	embeddingModel := c.Query("embedding_model", "text-embedding-3-small")

	// Load provider
	var provider database.LLMProvider
	providerIDStr := c.Query("provider_id")
	if providerIDStr != "" {
		pID, err := strconv.Atoi(providerIDStr)
		if err == nil {
			database.DB.Where("id = ? AND is_active = ?", pID, true).First(&provider)
		}
	}

	if provider.ID == 0 {
		database.DB.Where("is_active = ? AND is_embeddings = ?", true, true).First(&provider)
	}

	if provider.ID == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "No embeddings provider is configured. Please designate at least one active LLM provider for embeddings."})
	}

	// Determine embedding model: use provider's DefaultModel if non-empty, otherwise fallback
	if embeddingModel == "text-embedding-3-small" || embeddingModel == "" {
		if provider.DefaultModel != "" {
			embeddingModel = provider.DefaultModel
		} else {
			embeddingModel = "text-embedding-3-small"
			provName := strings.ToLower(provider.Name)
			provURL := strings.ToLower(provider.BaseURL)
			if strings.Contains(provName, "gemini") || strings.Contains(provURL, "gemini") {
				embeddingModel = "text-embedding-004"
			}
		}
	}

	masterKey := os.Getenv("DB_ENCRYPTION_KEY")
	apiKey, err := utils.Decrypt(provider.APIKey, masterKey)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to decrypt provider API key"})
	}

	llmClient := ai.NewClient(apiKey, provider.BaseURL)
	queryVector, err := llmClient.CreateEmbeddings(c.Context(), query, embeddingModel)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("failed to generate query embedding: %v", err)})
	}

	var chunks []database.DocumentChunk
	limit := 5
	err = database.DB.Order(gorm.Expr("embedding <=> ?", pgvector.NewVector(queryVector))).
		Limit(limit).
		Find(&chunks).Error

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("failed to search database: %v", err)})
	}

	type ChunkResult struct {
		ID        uint      `json:"id"`
		Source    string    `json:"source"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}

	var results []ChunkResult
	for _, chunk := range chunks {
		results = append(results, ChunkResult{
			ID:        chunk.ID,
			Source:    chunk.Source,
			Content:   chunk.Content,
			CreatedAt: chunk.CreatedAt,
		})
	}

	return c.JSON(fiber.Map{
		"query":   query,
		"results": results,
	})
}
