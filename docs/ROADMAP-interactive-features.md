# Roadmap: Capacidades Interactivas para Aigent

Este documento describe las features propuestas para transformar Aigent de un chat con herramientas MCP a una plataforma de aprendizaje interactivo multit model, con canvas de código, diagramas, quizzes y manejo de documentos.

---

## Contexto actual

- **Stack**: Go (Fiber) backend + Angular frontend, PostgreSQL (GORM), MCP (stdio + stream) para tools.
- **Arquitectura clave**: `Brain` orquesta el loop LLM → tools → respuesta. Los providers LLM son intercambiables (Zen, Groq, OpenRouter, OpenAI, custom) con fallback automático.
- **Chat actual**: Mensajes secuenciales (user → assistant → tool → assistant). Confirmación para tools sensitivas. Sin soporte para contenido rich más allá de texto plano y badges de tool calls.
- **Fortaleza**: Ya hay infraestructura MCP extensible (stdio/stream), sistema de agents con tools configurables, y fallback de providers. La plataforma está bien posicionada para agregar capacidades iterando sobre el stack existente.

---

## 1. Código Ejecutable (Code Sandbox)

### Idea
Permitir que el LLM genere código que se ejecute en un sandbox aislado y devuelva el resultado al chat, similar a los code interpreter de ChatGPT/Claude pero agnóstico de modelo.

### Enfoque recomendado: CubeSandbox (Tencent) via MCP

**¿Por qué CubeSandbox?**
- Es un MCP server oficial que provee ejecución de código en sandbox aislado (Docker + gVisor).
- Se integra nativamente con la infraestructura MCP que **ya existe** en Aigent (mcpstdio + mcpstream).
- Soporta múltiples lenguajes: Python, JavaScript, TypeScript, Go, Rust, etc.
- No requiere cambios en el backend Go, solo agregar el MCP server.

**Implementación:**

1. **Configuración como MCP stdio server** (lo más simple):
   ```
   Command: npx
   Args:    ["-y", "@anthropic/mcp-codeinterpreter", "--command", "node", "--arg", "sandbox.mjs"]
   ```
   O alternativas como `@anthropic/code-interpreter-mcp` o directamente CubeSandbox:
   ```
   Command: npx
   Args:    ["-y", "@anthropic/mcp-codeinterpreter"]
   ```

2. **CubeSandbox deployment propio** (más control):
   - Deploy del servicio CubeSandbox (`cubesandbox/server`) en Docker dentro del compose existente.
   - Registrar como MCP stream server apuntando a `http://cubesandbox:8080/mcp`.
   - Las tools `execute_code` y `run_snippet` aparecerán automáticamente en el registry de Aigent.

3. **Flujo UX**:
   - El agente genera código cuando el usuario lo pide.
   - La tool se ejecuta en el sandbox, el resultado (stdout, archivos generados, errores) vuelve como respuesta de tool al LLM.
   - El LLM puede iterar sobre errores y corregir.
   - En el frontend: renderItem especial para código ejecutado con syntax highlighting + panel de output.

### Cambios necesarios

| Capa | Cambio |
|------|--------|
| **Backend** | Ninguno (MCP ya soportado). Opcional: agregar preset de MCP stdio para CubeSandbox |
| **Frontend** | Nuevo componente de renderizado: código + output + estado (running/error/success) |
| **Docker** | Agregar `cubesandbox` al `docker-compose.yml` si se usa deployment propio |
| **BD** | Ninguno |

### Alternativa: evaluation engine nativo en Go

Si se quiere más control sin depender de MCP, se puede build un evaluator nativo:

- **Backend**: nuevo handler `HandleRunCode` que envíe código a un contenedor efímero (Docker API o Firecracker microVM).
- **Seguridad**: timeout, sin red, filesystem read-only, memory limit.
- **Ventaja**: control total sobre el entorno, sin dependencia externa.
- **Desventaja**: más trabajo, hay que maintain el runtime.

**Recomendación**: empezar con CubeSandbox via MCP. Iterar después con runtime propio si se necesita más control.

---

## 2. Canvas (iframe colaborativo tipo Gemini/ChatGPT Canvas)

### Idea
Un panel lateral o modal donde el usuario puede editar código o texto generado por el LLM, con cambios sincronizados en tiempo real hacia el chat. Similar a la experiencia de "Canvas" de ChatGPT o Gemini.

### Arquitectura propuesta

```
┌─────────────────────────────────────────────────┐
│ Chat                    │ Canvas                │
│                         │                       │
│ > Genera una función   │  def fibonacci(n):    │
│   fibonacci             │      if n <= 1:       │
│                         │          return n     │
│ [Ver en Canvas] ←──────│      return fib(n-1)  │
│                         │  + fib(n-2)           │
│ < Acá está tu función,  │                       │
│   podés editarla        │  [Run] [Copy] [Share] │
│   directamente          │                       │
└─────────────────────────────────────────────────┘
```

### Componentes

1. **CanvasPanel component** (Angular):
   - Split pane resizable (chat 60% / canvas 40%).
   - Editor con syntax highlighting (Monaco Editor o CodeMirror).
   - Soporta múltiples tabs: código, markdown, HTML preview.
   - Botones: Run (sandbox), Copy, Share al chat, Download.

2. **CanvasMessage type** en el chat:
   - Backend responde con `content_type: "canvas"` además de texto.
   - El LLM genera código + instrucciones de canvas, el backend lo parsea.
   - El frontend renderiza el canvas panel en vez de (o además de) texto plano.

3. **Sincronización bidireccional**:
   - El usuario edita en el canvas → se envía diff al chat como contexto adicional.
   - El LLM puede sugerir cambios que se aplican automáticamente al canvas.

### Cambios necesarios

| Capa | Cambio |
|------|--------|
| **Backend** | Agregar `CanvasArtifact` model (id, session_id, type, content, language) con CRUD handlers |
| **Backend** | Modificar `ChatMessage` para tener `content_type`: `text`, `canvas`, `diagram`, `quiz` |
| **Backend** | Nuevo handler `POST /sessions/:id/artifacts` y `GET /sessions/:id/artifacts` |
| **Frontend** | Nuevo componente `CanvasPanel` con Monaco Editor |
| **Frontend** | Modificar `Chat` para soportar split pane |
| **Frontend** | Nuevo servicio `CanvasService` en Angular |
| **Prompt** | Agregar system prompt指令 para que el LLM sepa cuándo usar canvas vs texto |

### Convención para el LLM

Agregar al system prompt:

```
Cuando generes código que el usuario pueda editar o ejecutar, envuélvelo en un artifact:
<artifact type="code" language="python" title="fibonacci">
def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)
</artifact>

Cuando generes contenido visual (diagramas, HTML), usa:
<artifact type="diagram" format="mermaid" title="Flujo del sistema">
graph TD
    A[Start] --> B[Process]
</artifact>
```

El backend parsea estos artifacts y los almacena. El frontend los renderiza según `type`.

---

## 3. Diagramas en el Chat

### Idea
Renderizar diagramas generados por el LLM directamente en el flujo del chat, sin necesidad de herramientas externas.

### Formatos soportados

1. **Mermaid** (recomendado como formato principal):
   - Flujos, secuencias, class diagrams, ER, Gantt, state, pie.
   - Renderer: `mermaid.js` (CDN o npm) desde Angular.
   - El LLM genera código Mermaid, el frontend lo renderiza inline.

2. **PlantUML** (alternativa):
   - Más expresivo para diagramas complejos.
   - Requiere server PlantUML para renderizar (Docker image `plantuml/plantuml-server`).
   - Agregar como MCP stream server para que el LLM lo use como tool.

3. **D2** (opcional):
   - Sintaxis moderna, declarativa.
   - Renderer: `@terrastruct/d2` o API externa.

### Implementación step by step

**Fase 1 — Mermaid inline (simplest, high ROI)**:
- Agregar `mermaid` al Angular (npm install mermaid).
- Nuevo componente `DiagramRenderer` que toma código Mermaid y lo renderiza a SVG.
- System prompt modificado: "Cuando el usuario pida un diagrama, generá código Mermaid dentro de un artifact `<artifact type='diagram' format='mermaid'>`".
- El backend parsea el artifact, frontend lo renderiza.

**Fase 2 — Interactividad**:
- Botón "Editar" que abre el código Mermaid en el Canvas panel.
- Botón "Descargar como PNG/SVG".
- Botón "Abrir en pantalla completa".

**Fase 3 — Diagramas como tool MCP**:
- Nuevo MCP stdio server que acepta `generate_diagram(type, content)` y devuelve la imagen/SVG.
- El LLM puede generar diagramas como respuesta de tool en vez de inline.

### Cambios necesarios (Fase 1)

| Capa | Cambio |
|------|--------|
| **Frontend** | `npm install mermaid`, nuevo `DiagramRenderer` component |
| **Frontend** | Modificar renderizado de mensajes para detectar artifacts de diagrama |
| **Backend** | Parser de `<artifact>` tags en `prompt_logic.go` o `chat_handler.go` |
| **Prompt** | Instrucciones de Mermaid en `buildSystemPromptForSession` |

---

## 4. Subida y Análisis de PDFs / Documentos

### Idea
Permitir al usuario subir PDFs, imágenes u otros documentos para que el LLM los analice, extraiga información, genere resúmenes, diagramas o quizzes.

### Arquitectura

```
User ──upload──→ POST /sessions/:id/attachments
                        │
                        ▼
                  ┌──────────────┐
                  │  File Store  │ (local / S3)
                  │  + Text Ext  │
                  └──────┬───────┘
                         │
              ┌──────────┼──────────┐
              ▼          ▼          ▼
         PDF Extract  Image OCR   Text Extract
         (pdfcpu     (Tesseract   (plain text)
          or pdftotext)  or API)
              │          │          │
              └──────────┼──────────┘
                         ▼
                  +─── Embedding ───→ Vector DB (pgvector)
                  │
                  ▼
            Context enrichment → LLM prompt
```

### Implementación

**Fase 1 — Subida básica + extracción de texto**:

1. **Modelo `Attachment`** en GORM:
   ```go
   type Attachment struct {
       ID        uint      `gorm:"primarykey" json:"id"`
       SessionID uint      `gorm:"not null;index" json:"session_id"`
       FileName  string    `gorm:"size:255" json:"file_name"`
       FileType  string    `gorm:"size:50" json:"file_type"` // pdf, png, txt, md, csv
       FilePath  string    `gorm:"size:1024" json:"file_path"`
       TextContent string  `gorm:"type:text" json:"text_content"` // texto extraído
       FileSize  int64     `json:"file_size"`
       CreatedAt time.Time `json:"created_at"`
   }
   ```

2. **Handler de upload** (`HandleUploadAttachment`):
   - Acepta multipart form.
   - Guarda archivo en `./uploads/` (o S3).
   - Si es PDF: extrae texto con `pdfcpu` o `pdftotext` (Go library o CLI).
   - Si es imagen: OCR con Tesseract o envío a un modelo de visión (el LLM mismo, vía multimodal).
   - Si es texto/md/csv: lectura directa.
   - Almacena `text_content` en la BD.

3. **Context injection**:
   - Al procesar un mensaje, si la sesión tiene attachments, se inyecta el `text_content` resumido en el system prompt o como contexto adicional.
   - Estrategia: "Documentos del usuario: {resumen de cada attachment}" dentro del prompt.

4. **Frontend**:
   - File input en el chat (drag & drop + botón).
   - Preview de archivos subidos (nombre, tamaño, tipo).
   - Indicador de procesamiento ("Analizando PDF...").

**Fase 2 — Vision multimodal**:
- Para PDFs con imágenes/gráficos, enviar las páginas como imágenes al LLM.
- Los providers que soportan vision (OpenAI GPT-4o, Gemini, etc.) pueden procesar imágenes directamente.
- Agregar `image_url` en los messages del API de chat para providers que lo soportan.

**Fase 3 — RAG con embeddings**:
- Para documentos largos: chunking + embeddings almacenados en pgvector.
- Retrieval por similitud semántica al momento de generar la respuesta.
- Permite consultas tipo "busca en el documento X la sección sobre Y".

### Cambios necesarios

| Capa | Cambio |
|------|--------|
| **BD** | Nueva tabla `attachments`, agregar extensión `pgvector` |
| **Backend** | Nuevo handler CRUD attachments, extracción de texto de PDFs/img |
| **Backend** | Modificar `ProcessChatInteraction` para inyectar contexto de attachments |
| **Frontend** | File upload component en chat, drag & drop |
| **Docker** | Agregar `pdftotext` o `tesseract` al container si se usa extracción local |
| **Prompt** | System prompt con instrucciones para manejar documentos adjuntos |

---

## 5. Quizzes Interactivos (Multiple Choice)

### Idea
El LLM genera preguntas de multiple choice inline en el chat. El usuario responde seleccionando una opción, y el sistema registra si es correcta, proporcionando feedback y seguimiento del progreso.

### Formato de quiz en el chat

```html
<quiz question="¿Cuál es la complejidad de búsqueda binaria?" type="multiple_choice">
  <option correct="false">O(n)</option>
  <option correct="true">O(log n)</option>
  <option correct="false">O(n²)</option>
  <option correct="false">O(1)</option>
  <explanation>
    La búsqueda binaria divide el espacio de búsqueda a la mitad en cada paso,
    resultando en una complejidad temporal de O(log n).
  </explanation>
</quiz>
```

### Arquitectura

1. **Backend — QuizArtifact**:
   - El LLM genera un quiz como artifact especial (`type="quiz"`).
   - El backend parsea el XML/JSON del quiz, lo valida, y lo almacena en la BD.
   - Nuevo handler `POST /sessions/:id/quiz/answer` que recibe `{quiz_id, selected_option}` y devuelve feedback.

2. **Frontend — QuizComponent**:
   - Renderiza pregunta + opciones como botones/cards clickeables.
   - Al responder: animación de acierto/error, muestra explicación.
   - Botón "Siguiente pregunta" o "Generar más preguntas sobre este tema".

3. **Seguimiento de progreso**:
   - Nuevo modelo `QuizResult`:
     ```go
     type QuizResult struct {
         ID           uint      `gorm:"primarykey" json:"id"`
         SessionID    uint      `gorm:"not null;index" json:"session_id"`
         QuizID       string    `json:"quiz_id"`
         Question     string    `json:"question"`
         SelectedIdx  int       `json:"selected_idx"`
         CorrectIdx   int       `json:"correct_idx"`
         IsCorrect    bool      `json:"is_correct"`
         CreatedAt    time.Time `json:"created_at"`
     }
     ```
   - Dashboard de progreso: % de aciertos por tema, racha, etc.

4. **System prompt para quizzes**:
   ```
   Cuando el usuario pida practicar o aprender un tema, generá quizzes interactivos usando el formato:
   <artifact type="quiz">
   {
     "question": "...",
     "options": ["A", "B", "C", "D"],
     "correct": 1,
     "explanation": "..."
   }
   </artifact>
   Generá preguntas que aumenten gradualmente de dificultad. Tras 3 respuestas correctas, subí el nivel.
   ```

5. **Interactividad sin modelo atado**:
   - El quiz se genera vía prompt del LLM (cualquier provider).
   - La validación de respuestas ocurre en el frontend (el artifact ya tiene la respuesta correcta).
   - No se necesita ningún modelo especial — cualquier LLM que siga instrucciones puede generar quizzes.
   - El feedback se genera enviando la respuesta del usuario como nuevo mensaje al LLM: "Respondí X, es correcto? Explicame por qué".

### Cambios necesarios

| Capa | Cambio |
|------|--------|
| **BD** | Nuevas tablas `quiz_results`, quiz_id como UUID en `chat_messages` |
| **Backend** | Parser de artifact tipo quiz en el chat handler |
| **Backend** | Nuevos endpoints: `POST /sessions/:id/quiz/answer`, `GET /sessions/:id/quiz/stats` |
| **Frontend** | Nuevo `QuizComponent` con animaciones y feedback |
| **Frontend** | Estadísticas de progreso por tema |
| **Prompt** | Instrucciones de formato de quiz en system prompt |

---

## 6. Análisis de Codebase (Subida de Repos / Code Review)

### Idea
El usuario puede compartir código fuente (archivos, snippets, o repos de GitHub) para que el LLM los analice, explique, genere diagramas de arquitectura, identify bugs, o cree quizzes basados en el código.

### Implementación

**Fase 1 — Snippets y archivos**:
- Ya cubierto por la subida de attachments (sección 4).
- El LLM recibe el código como contexto y lo analiza.

**Fase 2 — Repos de GitHub**:
- Nuevo handler `POST /sessions/:id/github-repo` con `{url, branch?}`.
- Backend clona el repo (shallow clone) o usa GitHub API para obtener archivos.
- Indexa los archivos principales (README, src/, config) como attachments de la sesión.
- El LLM analiza la estructura, genera diagramas de dependencias, explica patrones.

**Fase 3 — Code review interactivo**:
- El LLM genera anotaciones sobre líneas específicas.
- El usuario puede hacer follow-up preguntas sobre secciones del código.
- Se integran diagramas Mermaid generados a partir del análisis.

### Cambios necesarios

| Capa | Cambio |
|------|--------|
| **Backend** | Nuevo handler para clonar/indexar repos de GitHub |
| **Backend** | Modelo `CodebaseAttachment` (repo_url, branch, archivos indexados) |
| **Frontend** | Componente de visualización de código con anotaciones |
| **Docker** | Agregar `git` al container del backend |

---

## Plan de Implementación por Prioridad

### Fase 1 — Fundación (1-2 semanas)
1. **Infraestructura de artifacts**: modelo en BD + parser de `<artifact>` tags en el backend.
2. **Diagramas Mermaid**: component de renderizado + system prompt actualizado.
3. **CubeSandbox via MCP**: registrar MCP server, agregar preset en UI de MCP.

### Fase 2 — Interactividad (2-3 semanas)
4. **Quizzes múltiples choice**: componente + parsing + sistema de feedback.
5. **Subida de archivos/PDFs**: modelo Attachment + handler upload + extracción de texto.
6. **Canvas panel**: split pane + Monaco Editor + sincronización con chat.

### Fase 3 — Profundización (2-3 semanas)
7. **Vision multimodal**: enviar imágenes/PDFs como imágenes al LLM.
8. **RAG con embeddings**: pgvector + chunking + retrieval semántico.
9. **Repos de GitHub**: clone + index + análisis de codebase.

### Fase 4 — Pulido (1-2 semanas)
10. **Dashboard de progreso**: estadísticas de quizzes, temas dominados.
11. **Exportación**: descargar diagramas como PNG/SVG, exportar canvas.
12. **Temas de aprendizaje**: paths estructurados con quizzes secuenciales.

---

## Consideración Clave: Agnosticismo de Modelo

Todas las features están diseñadas para funcionar con **cualquier LLM** que soporte el provider actual:

- **Artifacts con tags XML**: el LLM genera contenido estructurado usando convenciones de prompt, no APIs propietarias.
- **Quizzes**: la lógica de validación está en el frontend (no depende de function calling específico).
- **Diagramas**: Mermaid es texto plano, cualquier LLM puede generarlo.
- **Sandbox**: es una tool MCP, se registra como cualquier otra tool del registry.
- **Canvas**: es un pattern de UI, el LLM solo necesita saber el formato `<artifact>`.

**Fallback strategy**: si un modelo no sigue bien el formato de artifacts, el fallback es mostrar el contenido como texto plano con botón "Abrir en canvas/editor".

---

## Dependencias técnicas sugeridas

| Dependencia | Uso | Dónde |
|-------------|-----|-------|
| `mermaid` (npm) | Renderizado de diagramas | Frontend |
| `@anthropic/mcp-codeinterpreter` o CubeSandbox | Ejecución de código | Docker / MCP |
| `monaco-editor` (npm) | Editor de código en canvas | Frontend |
| `pdfcpu` o `pdftotext` | Extracción de texto de PDFs | Backend (Go lib o CLI) |
| `pgvector` (extensión PostgreSQL) | Embeddings para RAG | Base de datos |
| `github.com/mmcdole/gofpdf` o `unidoc/unipdf` | Generación de PDFs (export) | Backend |
| `github.com/disintegration/imaging` | Procesamiento de imágenes | Backend |

---

## Resumen visual de la arquitectura propuesta

```
┌─────────────────────────────────────────────────────────────────┐
│                        FRONTEND (Angular)                        │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │  Chat     │  │ Canvas   │  │ Diagram  │  │  Quiz         │  │
│  │  Panel    │  │ Editor   │  │ Renderer │  │  Component    │  │
│  │          │  │ (Monaco) │  │ (Mermaid)│  │  (MC)        │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────┬────────┘  │
│       │              │              │               │           │
│  ┌────┴──────────────┴──────────────┴───────────────┴────┐      │
│  │                  Artifact Renderer                      │      │
│  │       (parsea <artifact type="..."> y despacha)      │      │
│  └───────────────────────┬───────────────────────────────┘      │
│                          │ API                                    │
└──────────────────────────┼───────────────────────────────────────┘
                           │
┌──────────────────────────┼───────────────────────────────────────┐
│                     BACKEND (Go/Fiber)                           │
│                          │                                       │
│  ┌───────────────────────┴───────────────────────────────┐      │
│  │                  Chat Handler                           │      │
│  │  • Parse <artifact> tags en respuesta del LLM          │      │
│  │  • Almacenar artifacts en BD                            │      │
│  │  • Inyectar contexto de attachments en prompt           │      │
│  └───────┬───────────────────────┬───────────────────────┘      │
│          │                       │                               │
│  ┌───────┴────────┐  ┌──────────┴──────────┐                    │
│  │  Brain (LLM)   │  │  MCP Registry        │                    │
│  │  • Providers   │  │  • CubeSandbox (code) │                    │
│  │  • Fallback    │  │  • PlantUML (diagram) │                    │
│  │  • Prompts     │  │  • HandsAI tools      │                    │
│  └────────────────┘  └──────────────────────┘                    │
│                                                                  │
│  ┌──────────────────┐  ┌──────────────────┐                     │
│  │  Attachment Svc  │  │  Quiz Service     │                     │
│  │  • Upload        │  │  • Validate       │                     │
│  │  • Text extract  │  │  • Stats          │                     │
│  │  • PDF parsing   │  │  • Results        │                     │
│  └──────────────────┘  └──────────────────┘                     │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
                           │
                    ┌──────┴──────┐
                    │ PostgreSQL  │
                    │ + pgvector  │
                    └─────────────┘
```