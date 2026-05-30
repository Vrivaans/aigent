# Propuesta de Arquitectura: Capacidades Interactivas y Aprendizaje en Aigent

Esta propuesta detalla el diseño técnico y de experiencia de usuario (UX) para transformar **Aigent** de un asistente de chat convencional a una plataforma interactiva de aprendizaje y desarrollo. 

La meta principal es brindar capacidades de ejecución de código, edición colaborativa (Canvas), diagramación dinámica, análisis de documentos y codebase, y evaluaciones interactivas (Quizzes), todo de manera **agnóstica al modelo de lenguaje (LLM)**.

---

## 1. Arquitectura Agnóstica y Flujo de Comunicación

Para garantizar que el sistema funcione con cualquier LLM (Gemini, Claude, GPT, DeepSeek, etc.) sin depender de APIs específicas de "artifacts" o "canvas" de una plataforma en particular, utilizaremos un **protocolo de marcado basado en XML/Tags** inyectado a través del System Prompt y procesado en las dos capas de Aigent:

```
┌─────────────────┐       XML Artifacts       ┌──────────────────┐
│   Cualquier     ├──────────────────────────>│  Backend en Go   │
│   LLM (Brain)   │  (Genera texto con tags)  │  (Fiber Handler) │
└─────────────────┘                           └────────┬─────────┘
                                                       │
                                                       │ Parsea tags y guarda en DB
                                                       │ Envía JSON estructurado
                                                       ▼
                                              ┌──────────────────┐
                                              │ Frontend Angular │
                                              │ (Renderizadores) │
                                              └──────────────────┘
```

### Protocolo de Marcado (System Prompt)
El orquestador (`Brain`) inyectará reglas claras de formato. El LLM generará bloques especiales en sus respuestas de texto:

```xml
<artifact type="canvas" language="go" title="main.go" id="art-123">
package main
import "fmt"
func main() {
    fmt.Println("Hola, Aigent!")
}
</artifact>

<artifact type="diagram" format="mermaid" id="diag-456" title="Arquitectura Clean">
graph TD
    A[Frontend] -->|API| B[Go Backend]
    B -->|GORM| C[(PostgreSQL)]
</artifact>

<artifact type="quiz" id="quiz-789">
{
  "question": "¿Cuál es la complejidad temporal de búsqueda binaria?",
  "options": ["O(n)", "O(log n)", "O(n log n)", "O(1)"],
  "correct_index": 1,
  "explanation": "La búsqueda binaria divide el espacio de búsqueda por la mitad en cada paso, reduciendo el problema a escala logarítmica."
}
</artifact>
```

### Procesamiento en Backend (Go/Fiber)
1. El backend intercepta el stream de respuesta del LLM.
2. Utiliza un parser reactivo de tags XML/JSON (mediante regex de streaming o lexer simple) para separar el texto conversacional del contenido del artefacto.
3. Almacena el artefacto en la base de datos (`Artifact`, `Diagram`, `Quiz`) vinculado a la sesión.
4. Envía al frontend un payload limpio: texto conversacional + una lista de referencias a artefactos estructurados.

---

## 2. Código Ejecutable (Code Sandbox)

Para permitir la ejecución interactiva de código sin comprometer la seguridad del servidor de la aplicación, implementaremos una estrategia basada en **MCP (Model Context Protocol)** extensible a ejecuciones locales controladas.

```
┌──────────────────┐    Run Snippet    ┌─────────────────┐   Crea sandbox   ┌────────────────────┐
│   Web Frontend   ├──────────────────>│   Go Backend    ├─────────────────>│  MCP CubeSandbox   │
│ (Monaco Editor)  │  (WS / REST API)  │ (MCP Registry)  │ (Docker/gVisor)  │ (Aislado de Red)   │
└──────────────────┘                   └─────────────────┘                  └────────────────────┘
```

### Arquitectura de Ejecución
1. **MCP Server (Tencent CubeSandbox / Anthropic Code Interpreter)**:
   - **Configuración Externa / Manual**: El usuario realiza el despliegue del sandbox de Tencent (por ejemplo, en un contenedor Docker local) y lo registra en `mcp_config.json`.
   - **Aprovechamiento de la Infraestructura Existente**: Dado que Aigent ya soporta la integración con servidores MCP (stdio y stream), no es necesario desarrollar lógica de sandboxing o aislamiento de procesos en el core de Aigent.
   - Provee la tool `execute_code(language, code, files)`.
2. **Flujo de Ejecución**:
   - El LLM escribe código en un bloque `canvas`.
   - El frontend de Aigent renderiza un botón **"Run"** sobre el Monaco Editor del Canvas.
   - Al hacer clic en "Run", el frontend invoca al backend de Aigent, el cual reenvía la petición a la tool del MCP configurado. El backend simplemente actúa como puente y devuelve el output (`stdout`, `stderr`, archivos) al frontend para mostrarlo en la consola interactiva.

---

## 3. Canvas Colaborativo (Split-Screen UX)

El canvas es un espacio dedicado a la iteración sobre código, markdown o previsualizaciones HTML, imitando la experiencia de Claude Artifacts o Gemini Canvas.

```
┌──────────────────────────────────────┬──────────────────────────────────────┐
│ PANEL DE CHAT (60%)                  │ PANEL DE CANVAS (40%)                │
│                                      │                                      │
│ User: Ayúdame con una función Go.    │  main.go                     [Run]   │
│                                      │ ┌──────────────────────────────────┐ │
│ Aigent: He creado la función.        │ │ 1 │ package main                 │ │
│ Podés verla y ejecutarla a la        │ │ 2 │ func sum(a, b int) int {     │ │
│ derecha.                             │ │ 3 │     return a + b             │ │
│                                      │ │ 4 │ }                            │ │
│ [Botón: Abrir en Canvas]             │ └──────────────────────────────────┘ │
│                                      │  Output:                             │ │
│                                      │  [Running...]                        │ │
│                                      │  Result: 12                          │ │
└──────────────────────────────────────┴──────────────────────────────────────┘
```

### Sincronización Bidireccional
- **LLM -> Canvas**: Cada vez que el LLM genera o actualiza un bloque de código, el componente Angular actualiza el editor Monaco en la pestaña correspondiente sin recargar todo el chat.
- **User -> Canvas**: El usuario puede editar el código libremente en el Monaco Editor.
- **Canvas -> LLM**: Se añade un botón de acción en el Canvas (ej. **"Mejorar código"**, **"Explicar"**, o **"Refactorizar"**). Al presionarlo, se envía un mensaje automático invisible al chat con el contenido actual del Canvas para que el LLM proponga los cambios.
- **Control de Versiones**: En la base de datos, los cambios en los artefactos se guardan con historial (`ArtifactVersion` model con `version_number`, `diff`, `created_at`), permitiendo al usuario volver atrás en cualquier paso de su aprendizaje.

---

## 4. Diagramas Dinámicos e Interactivos

La capacidad de visualizar flujos facilita enormemente el aprendizaje de sistemas y código base.

```
┌────────────────────────────────────────────────────────┐
│ diagram.mermaid                                 [Edit] │
├────────────────────────────────────────────────────────┤
│                                                        │
│   ┌──────────────┐       API       ┌────────────────┐  │
│   │   Frontend   ├────────────────>│   Go Backend   │  │
│   └──────────────┘                 └───────┬────────┘  │
│                                            │           │
│                                            ▼           │
│                                    ┌────────────────┐  │
│                                    │  PostgreSQL    │  │
│                                    └────────────────┘  │
│                                                        │
└────────────────────────────────────────────────────────┘
```

### Implementación y Renderizado
1. **Mermaid.js Integrado**: El frontend de Angular incluirá la librería oficial de Mermaid. Cuando el parser detecte un artifact de tipo `diagram` con formato `mermaid`, renderizará el SVG de forma dinámica en un panel de diagrama dedicado.
2. **Capas Interactivas (Click to Query)**:
   - Al renderizar el SVG, inyectaremos selectores en los nodos.
   - El usuario podrá hacer **clic en cualquier nodo** del diagrama. Esto abrirá un menú contextual: *"¿Qué hace este componente?"* o *"Generar código para esta parte"*.
   - Al elegir, el frontend enviará una pregunta contextual al chat (ej: *"Háblame más sobre el nodo [Go Backend] en el diagrama anterior y cómo procesa las peticiones"*).
3. **Alternativas avanzadas (Vía MCP)**: Si se requieren diagramas UML específicos o D2, se delega al 100% en MCP servers externos que ya resuelven este renderizado. No incluiremos soporte nativo para PlantUML o D2 en el core del backend de Aigent, manteniendo la aplicación ligera. Nos enfocaremos únicamente en Mermaid.js en el frontend (que corre directo en el navegador) y en renderizar los archivos de imagen/SVG devueltos por herramientas MCP de diagramación.

---

## 5. Análisis de PDFs, Documentos y Codebase (RAG)

El aprendizaje interactivo requiere que el usuario pueda proveer material externo (documentos de estudio, APIs, o repositorios enteros) para luego interactuar con ellos.

```
                  ┌───────────────────┐
                  │ Upload Document / │
                  │ GitHub Repository │
                  └─────────┬─────────┘
                            │
                            ▼
                  ┌───────────────────┐
                  │ Go Ingestion Svc  │
                  └─────────┬─────────┘
                            ├──────────────────────────┐
                            ▼                          ▼
                  ┌───────────────────┐      ┌───────────────────┐
                  │   Text / Code     │      │ Visual Components │
                  │    Extract        │      │    (Multimodal)   │
                  └─────────┬─────────┘      └─────────┬─────────┘
                            │                          │
                            ▼                          ▼
                  ┌───────────────────┐      ┌───────────────────┐
                  │ Chunking & Embed  │      │ Direct Attachment │
                  │ (pgvector DB)     │      │ to LLM Vision     │
                  └───────────────────┘      └───────────────────┘
```

### Procesamiento de Documentos (PDF, TXT, MD)
- **Extracción de Texto**: En el backend de Go, procesaremos los PDFs usando librerías nativas (`pdfcpu` o adaptando CLI de `pdftotext`). El texto limpio se almacena.
- **Estrategia RAG**:
  - Para documentos pequeños (< 20KB), inyectamos el contenido entero en el System Prompt de la sesión como contexto temporal.
  - Para documentos grandes o libros enteros, implementaremos chunking recursivo y almacenamiento de vectores en PostgreSQL usando la extensión **pgvector**. Al preguntar, se realiza una búsqueda de similitud semántica.
- **Multimodalidad**: Si el LLM seleccionado por el usuario soporta visión (ej. GPT-4o, Gemini 1.5 Pro) y sube una imagen o PDF con gráficos, el frontend permitirá adjuntar los archivos multimedia directamente en el array de inputs del LLM.

### Análisis de Codebase (GitHub / Local)
- **Ingesta de Repositorios**: Un endpoint `POST /sessions/:id/codebase/github` clonará un repositorio (shallow clone, limitando tamaño de archivos) o leerá archivos subidos en un zip.
- **Análisis y Mapeo**: El backend generará un índice plano del codebase (estructura de directorios, firmas de funciones principales). El LLM podrá usar este mapa para crear diagramas de flujo completos del sistema o explicar la interacción de componentes.

---

## 6. Quizzes Interactivos (Progreso y Gamificación)

Para validar el aprendizaje de forma activa, el LLM puede retar al usuario a resolver preguntas de opción múltiple estructuradas basadas en el tema actual o en los documentos subidos.

```
┌────────────────────────────────────────────────────────┐
│ TEST DE AUTOEVALUACIÓN                         [1 / 3] │
├────────────────────────────────────────────────────────┤
│ ¿Cuál de los siguientes no es un estado válido de un   │
│ canal en Go?                                           │
│                                                        │
│  [ ] Nil (Lecturas bloquean)                           │
│  [ ] Abierto                                           │
│  [x] Cerrado (Lecturas retornan valor cero)            │
│  [ ] Pausado                                           │
│                                                        │
│ ────────────────────────────────────────────────────── │
│ [x] ¡Correcto!                                         │
│ Explicación: Los canales en Go no tienen un estado     │
│ "Pausado". Solo pueden estar en nil, abiertos o        │
│ cerrados.                                              │
└────────────────────────────────────────────────────────┘
```

### Mecánica de los Quizzes
1. **Generación**: El LLM responde con un JSON structured dentro de la tag `<artifact type="quiz">`.
2. **Interactividad Local (Frontend)**: 
   - El frontend detecta el quiz y muestra las opciones como botones interactivos en lugar de texto plano.
   - El usuario selecciona la respuesta.
   - Al marcarla, se revela la respuesta correcta con una animación estilizada (verde para acierto, rojo para error) y la explicación detallada.
3. **Persistencia y Progreso**:
   - Las respuestas se envían a un endpoint del backend: `POST /sessions/:id/quizzes/:quiz_id/answer` con la opción seleccionada.
   - La base de datos registrará los resultados en la tabla `quiz_results` (ID de sesión, pregunta, correcta/incorrecta, tema, fecha).
4. **Retroalimentación Dinámica**:
   - Con base en la tasa de acierto en la base de datos, el orquestador puede ajustar el prompt dinámicamente: *"El usuario ha fallado 2 preguntas sobre punteros. Baja la complejidad y enfócate en explicar ese concepto básico antes de avanzar"*.

---

## 7. Modelos de Base de Datos Propuestos (GORM / PostgreSQL)

Para soportar estas características, agregaremos las siguientes estructuras en la base de datos de Go:

```go
// Artifact representa código, diagramas, HTML o markdown mutable.
type Artifact struct {
    ID          string    `gorm:"primaryKey;size:64" json:"id"`
    SessionID   uint      `gorm:"not null;index" json:"session_id"`
    Type        string    `gorm:"size:30" json:"type"` // canvas, diagram
    Title       string    `gorm:"size:255" json:"title"`
    Language    string    `gorm:"size:50" json:"language,omitempty"` // go, python, js, markdown
    Content     string    `gorm:"type:text" json:"content"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// ArtifactVersion guarda el historial de ediciones en Canvas.
type ArtifactVersion struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    ArtifactID  string    `gorm:"not null;index;size:64" json:"artifact_id"`
    Version     int       `gorm:"not null" json:"version"`
    Content     string    `gorm:"type:text" json:"content"`
    Author      string    `gorm:"size:20" json:"author"` // "user" o "ai"
    CreatedAt   time.Time `json:"created_at"`
}

// Attachment representa archivos PDF o imágenes analizadas.
type Attachment struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    SessionID   uint      `gorm:"not null;index" json:"session_id"`
    FileName    string    `gorm:"size:255" json:"file_name"`
    FileType    string    `gorm:"size:50" json:"file_type"` // pdf, png, txt, zip
    FilePath    string    `gorm:"size:1024" json:"file_path"`
    TextContent string    `gorm:"type:text" json:"text_content"` // Extraído para inyección de contexto
    FileSize    int64     `json:"file_size"`
    CreatedAt   time.Time `json:"created_at"`
}

// QuizResult registra la gamificación y rendimiento del usuario.
type QuizResult struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    SessionID   uint      `gorm:"not null;index" json:"session_id"`
    QuizID      string    `gorm:"size:64;index" json:"quiz_id"`
    Question    string    `gorm:"type:text" json:"question"`
    IsCorrect   bool      `json:"is_correct"`
    SelectedIdx int       `json:"selected_idx"`
    CorrectIdx  int       `json:"correct_idx"`
    Topic       string    `gorm:"size:100;index" json:"topic"`
    CreatedAt   time.Time `json:"created_at"`
}
```

---

## 8. Plan de Adopción e Implementación

### Fase 1: Visualización y Diagramas (MVP de Lectura)
- Integrar **Mermaid.js** en el frontend.
- Programar el parser de tags `<artifact type="diagram">` en el streaming del backend en Go.
- Modificar el System Prompt para habilitar la diagramación en Mermaid de forma agnóstica.

### Fase 2: Interactividad del Canvas y Edición (Escritura)
- Integrar **Monaco Editor** en un panel lateral deslizante (Split-Screen).
- Diseñar el flujo de mensajería bidireccional (enviar diffs del editor al chat).
- Crear las tablas de base de datos para almacenar y versionar artefactos.

### Fase 3: Sandbox y Ejecución de Código
- Desplegar **CubeSandbox** como un servicio Docker local e integrarlo como MCP Stream Server.
- Añadir el botón "Run" en el panel Canvas que invoque el endpoint de ejecución.
- Mostrar la consola de salida interactiva debajo del editor.

### Fase 4: Carga de Documentos (RAG) y Gamificación (Quizzes)
- Implementar la carga y extracción de texto de PDFs.
- Integrar la tabla de Quizzes, el parser de quizzes JSON en el backend, y el componente visual de opción múltiple con estadísticas en Angular.
