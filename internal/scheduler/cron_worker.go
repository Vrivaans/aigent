package scheduler

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"aigent/internal/ai"
	"aigent/internal/database"
)

// StartCronWorker initializes a background process that sweeps the DB every minute.
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

	// Obtenemos tareas donde NextRunAt sea <= ahora (o sea NULL, forzando un primer run)
	if err := database.DB.Where("next_run_at <= ? OR next_run_at IS NULL", now).Find(&tasks).Error; err != nil {
		log.Printf("❌ Cron Worker Database Error: %v", err)
		return
	}

	for _, task := range tasks {
		log.Printf("⚙️ Cron executing task: '%s' invoking tool: '%s'", task.Name, task.ToolName)

		var args map[string]interface{}
		if err := json.Unmarshal(task.Payload, &args); err != nil {
			log.Printf("❌ Task '%s' failed to parse JSON payload: %v", task.Name, err)
			continue
		}

		// Ejecutamos la herramienta en HandsAI
		_, err := brain.HandsAI.CallTool(ctx, task.ToolName, args)
		if err != nil {
			log.Printf("❌ Task '%s' Execution Failed: %v", task.Name, err)
		} else {
			log.Printf("✅ Task '%s' Execution Succeeded", task.Name)
		}

		// Para este MVP, si arranca con un simple "crontab" de cada hora, lo sumamos, si no por 24hs.
		// En produccion usariamos un paquete como robfig/cron/v3 para parsear "0 9 * * *".
		nextRun := calculateNextRun(task.CronExpression, now)
		task.NextRunAt = &nextRun

		if err := database.DB.Save(&task).Error; err != nil {
			log.Printf("❌ Scheduled task DB update failed: %v", err)
		}
	}
}

func calculateNextRun(cron string, now time.Time) time.Time {
	cronLower := strings.ToLower(cron)
	if strings.Contains(cronLower, "hourly") || strings.Contains(cronLower, "@hourly") {
		return now.Add(time.Hour)
	}
	if strings.Contains(cronLower, "* * * * *") { 
		return now.Add(time.Minute)
	}
	// Por defecto 24 hrs
	return now.Add(24 * time.Hour)
}
