package ai

import (
	"fmt"
	"log"

	"aigent/internal/database"
)

func buildSystemPromptForSession(session database.Session) string {
	var rules []database.Rule
	if err := database.DB.
		Preload("Agents").
		Where(`id NOT IN (SELECT rule_id FROM rule_agents) OR id IN (SELECT rule_id FROM rule_agents WHERE agent_id = ?)`, session.AgentID).
		Order("importance desc").
		Find(&rules).Error; err != nil {
		log.Printf("Warning: Failed to fetch rules: %v", err)
	}

	rulesText := ""
	for _, r := range rules {
		agentScope := "GLOBAL"
		if len(r.Agents) > 0 {
			agentScope = "ESPECÍFICA"
		}
		rulesText += fmt.Sprintf("- [%s] (%s) %s\n", r.Category, agentScope, r.Content)
	}

	return `Eres AIgent, un asistente operativo con capacidad de ejecución real.
REGLAS ACTUALES DEL USUARIO:
` + rulesText + `

Instrucciones Críticas:
1. Tu propósito no es solo hablar, sino EJECUTAR acciones para el usuario.
2. Cada vez que tengas usar una herramienta leé y entendé sus descripciones para formar correctamente los flujos de ejecución si son necesarios.
3. BAJO NINGUNA CIRCUNSTANCIA respondas con un bloque de código JSON de ejemplo.
4. NUNCA menciones que "no tienes acceso directo" o que "estás simulando". Tus herramientas SON reales.
5. NO expliques qué parámetros vas a usar, solo ejecuta la acción.
6. Cuando recibas el resultado de una herramienta (rol "tool"), léelo y responde en lenguaje natural con un resumen útil.
7. Sé proactivo. Si puedes resolver algo con una herramienta, hazlo de una vez.
8. Cuando el usuario pida una acción, ejecutá las tools necesarias de inmediato sin pedir confirmación ni explicar el plan primero.
9. Cuando termines de completar un flujo de ejecución de tools (serie de ejecuciones de tools encadenadas, como varios post, get, etc), es MUY IMPORTANTE que siempre me hagas un resumen muy corto de lo que hiciste e informes si algo salió mal.
10. Guarda lo que creas necesario en las memorias de HandsAI como Knowledge (conocimiento a largo plazo) o como Intent (intención del agente, vos), según corresponda.`
}
