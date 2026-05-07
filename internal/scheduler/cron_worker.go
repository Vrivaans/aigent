package scheduler

import (
	"context"
	"log"
	"time"

	"aigent/internal/ai"
	"aigent/internal/database"
	tasksvc "aigent/internal/tasks"
)

func StartCronWorker(ctx context.Context, brain *ai.Brain) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Println("⏱️ Cron Worker started: Polling for scheduled tasks...")

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Cron Worker shutting down...")
			return
		case <-ticker.C:
			processScheduledTasks(ctx, brain)
		}
	}
}

func processScheduledTasks(ctx context.Context, brain *ai.Brain) {
	now := time.Now()
	var tasks []database.Task

	if err := database.DB.Where("next_run_at <= ? OR next_run_at IS NULL", now).Find(&tasks).Error; err != nil {
		log.Printf("❌ Cron Worker Database Error: %v", err)
		return
	}

	for _, task := range tasks {
		log.Printf("⚙️ Cron executing task: '%s' with agent %d", task.Name, task.AgentID)

		session := database.Session{
			Title:   "Cron: " + task.Name,
			AgentID: task.AgentID,
		}
		if err := database.DB.Create(&session).Error; err != nil {
			log.Printf("❌ Task '%s' failed to create session: %v", task.Name, err)
			task.LastError = "Failed to create session: " + err.Error()
			scheduleNextRun(&task, now)
			continue
		}

		userMsg := database.ChatMessage{
			SessionID: session.ID,
			Role:      "user",
			Content:   task.Prompt,
		}
		database.DB.Create(&userMsg)

		var history []database.ChatMessage
		history = append(history, userMsg)

		respMsg, intermediates, err := brain.ProcessChatInteraction(ctx, session.ID, history, "")
		nowAfter := time.Now()
		task.LastRunAt = &nowAfter

		if err != nil {
			log.Printf("❌ Task '%s' Execution Failed: %v", task.Name, err)
			task.LastError = err.Error()
			task.LastResult = ""
		} else {
			for i := range intermediates {
				database.DB.Create(&intermediates[i])
			}
			asstMsg := database.ChatMessage{
				SessionID:    session.ID,
				Role:         "assistant",
				Content:      respMsg.Content,
				RawToolCalls: "",
			}
			database.DB.Create(&asstMsg)

			log.Printf("✅ Task '%s' Execution Succeeded: %s", task.Name, truncate(respMsg.Content, 100))
			task.LastError = ""
			task.LastResult = respMsg.Content
		}

		scheduleNextRun(&task, now)
	}
}

func scheduleNextRun(task *database.Task, now time.Time) {
	if task.OneShot {
		log.Printf("🗑️ Deleting one-shot task '%s' after execution", task.Name)
		if err := database.DB.Delete(task).Error; err != nil {
			log.Printf("❌ Failed to delete one-shot task '%s': %v", task.Name, err)
		}
		return
	}
	nextRun := tasksvc.CalculateNextRun(task.CronExpression, now)
	task.NextRunAt = &nextRun

	if err := database.DB.Save(task).Error; err != nil {
		log.Printf("❌ Scheduled task DB update failed: %v", err)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
