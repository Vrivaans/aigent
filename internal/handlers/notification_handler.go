package handlers

import (
	"time"

	"aigent/internal/database"

	"github.com/gofiber/fiber/v2"
)

// ─────────────────────────────────────────────────────────────────────────────
// Centro de notificaciones: mensajes de cualquier agente al usuario, con link
// a la sesión que los originó para continuar en contexto desde la UI.
// ─────────────────────────────────────────────────────────────────────────────

type NotificationResponse struct {
	database.Notification
	SessionTitle string `json:"session_title,omitempty"`
}

// GetNotifications lista notificaciones del tenant, más recientes primero.
// Query: ?unread=true filtra no leídas, ?limit=N (default 50).
func GetNotifications(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	if limit < 1 || limit > 200 {
		limit = 50
	}

	query := database.DB.Model(&database.Notification{})
	if c.Query("unread") == "true" {
		query = query.Where("read_at IS NULL")
	}

	var notifications []database.Notification
	if err := query.Order("created_at desc").Limit(limit).Find(&notifications).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// títulos de sesión para el link contextual
	sessionIDs := map[uint]bool{}
	for _, n := range notifications {
		if n.SessionID > 0 {
			sessionIDs[n.SessionID] = true
		}
	}
	titles := map[uint]string{}
	if len(sessionIDs) > 0 {
		var sessions []database.Session
		ids := make([]uint, 0, len(sessionIDs))
		for id := range sessionIDs {
			ids = append(ids, id)
		}
		database.DB.Where("id IN ?", ids).Find(&sessions)
		for _, s := range sessions {
			titles[s.ID] = s.Title
		}
	}

	resp := make([]NotificationResponse, 0, len(notifications))
	for _, n := range notifications {
		resp = append(resp, NotificationResponse{Notification: n, SessionTitle: titles[n.SessionID]})
	}
	return c.JSON(resp)
}

// GetUnreadCount devuelve cuántas notificaciones no leídas hay (para el badge).
func GetUnreadCount(c *fiber.Ctx) error {
	var count int64
	if err := database.DB.Model(&database.Notification{}).Where("read_at IS NULL").Count(&count).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"unread": count})
}

// MarkNotificationRead marca una notificación como leída.
func MarkNotificationRead(c *fiber.Ctx) error {
	id := c.Params("id")
	res := database.DB.Model(&database.Notification{}).Where("id = ?", id).
		Update("read_at", time.Now())
	if res.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": res.Error.Error()})
	}
	if res.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "notification not found"})
	}
	return c.JSON(fiber.Map{"status": "read"})
}

// MarkAllNotificationsRead marca todas como leídas.
func MarkAllNotificationsRead(c *fiber.Ctx) error {
	if err := database.DB.Model(&database.Notification{}).Where("read_at IS NULL").
		Update("read_at", time.Now()).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "all read"})
}

// GetSessionByID devuelve una sesión individual. Necesario para abrir
// notificaciones de sesiones ocultas por filtro en la UI (cron/workflow).
func GetSessionByID(c *fiber.Ctx) error {
	session, err := loadSessionForTenant(c, c.Params("id"))
	if err != nil {
		return respondFiberError(c, err)
	}
	return c.JSON(session)
}

