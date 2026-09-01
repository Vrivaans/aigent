package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"aigent/internal/database"
)

// ─────────────────────────────────────────────────────────────────────────────
// Notificaciones usuario↔agente: cualquier agente puede avisar al usuario
// (propuestas enviadas, hallazgos, tareas completadas). Cada notificación
// linkea la sesión que la originó para continuar en contexto desde la UI.
// ─────────────────────────────────────────────────────────────────────────────

type ctxKeyNotification string

const notificationSessionIDKey ctxKeyNotification = "notification_session_id"

// withNotificationSession inyecta el sessionID en el ctx de ejecución de tools.
func withNotificationSession(ctx context.Context, sessionID uint) context.Context {
	return context.WithValue(ctx, notificationSessionIDKey, sessionID)
}

// sessionIDFromCtx recupera el sessionID inyectado (0 si no hay).
func sessionIDFromCtx(ctx context.Context) uint {
	if v, ok := ctx.Value(notificationSessionIDKey).(uint); ok {
		return v
	}
	return 0
}

func registerNotificationTools(b *Brain) {
	b.Registry.Register(ToolDef{
		Name: "notify_user",
		Description: "Notifica al usuario un evento importante: propuesta enviada, hallazgo relevante, tarea completada, alerta. La notificación aparece en el centro de notificaciones de la UI y linkea esta sesión para continuar el contexto. Usala al cerrar hitos, no para pasos intermedios triviales.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"title":{"type":"string","description":"Título corto (ej: 'Propuesta enviada: Stripe Integration')"},"message":{"type":"string","description":"Detalle legible: qué pasó, qué sigue, qué se espera del usuario si aplica"},"level":{"type":"string","enum":["info","success","warning"],"description":"info=default, success=logros, warning=requiere atención"}},"required":["title","message"]}`),
		Execute: func(ctx context.Context, args map[string]interface{}) (json.RawMessage, error) {
			title, _ := args["title"].(string)
			message, _ := args["message"].(string)
			level, _ := args["level"].(string)
			if title == "" || message == "" {
				return nil, fmt.Errorf("title y message son requeridos")
			}
			if level == "" {
				level = "info"
			}
			if level != "info" && level != "success" && level != "warning" {
				return nil, fmt.Errorf("level inválido: %s (info|success|warning)", level)
			}
			n := database.Notification{
				SessionID: sessionIDFromCtx(ctx),
				Title:     title,
				Body:      message,
				Level:     level,
			}
			if err := database.DB.Create(&n).Error; err != nil {
				return nil, err
			}
			log.Printf("🔔 Notificación #%d (sesión %d): %s", n.ID, n.SessionID, title)
			return json.Marshal(map[string]interface{}{
				"notification_id": n.ID,
				"session_id":      n.SessionID,
				"message":         "Notificación creada. El usuario la verá en el centro de notificaciones.",
			})
		},
		Sensitive: false,
	})
}
