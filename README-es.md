# AIgent: El Orquestador de Agentes Digitales 🤖🚀

[![Hackaton CubePath 2026](https://img.shields.io/badge/Hackaton-CubePath_2026-blueviolet?style=for-the-badge)](https://github.com/midudev/hackaton-cubepath-2026)
[![Powered by Go](https://img.shields.io/badge/Powered_by-Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Angular 21](https://img.shields.io/badge/Angular-21-DD0031?style=for-the-badge&logo=angular&logoColor=white)](https://angular.io/)

**[English](README.md)** · Español

**AIgent** es un operador diseñado para actuar como un puente seguro y resiliente entre el usuario y sus herramientas de trabajo (operando principalmente las tools de HandsAI). A diferencia de los chatbots tradicionales, AIgent no solo habla: **ejecuta**. El propósito es que el agente pueda operar software de terceros de forma autónoma y segura.

---

## 🎬 Demo y capturas

| | |
|:--|:--|
| **Video (YouTube)** | [Demo en vivo — ejecución de herramientas en tiempo real](https://youtu.be/N7zXwUHNL5k) |

### Interfaz y flujo principal

#### Chat: el agente ejecuta herramientas en tiempo real (filesystem)

![Chat: el agente puede operar el filesystem](docs/img/vista%20chat%20puede%20operar%20filesystem.png)

#### Agentes especializados

![Vista de agentes](docs/img/vista%20de%20agentes.png)

#### Reglas para el comportamiento del agente

![Vista de reglas](docs/img/vista%20de%20reglas%20para%20agentes.png)

#### Proveedores de LLM

![Vista de proveedores LLM](docs/img/vista%20de%20proveedores%20llm.png)

#### Catálogo de herramientas

![Vista del catálogo de tools](docs/img/vista%20de%20tools.png)

### Integración MCP (stdio y HTTP streamable)

#### Nuevo servidor MCP stdio registrado y detectado

![Nuevo servidor MCP stdio detectado](docs/img/vista%20nuevo%20servidor%20mcp%20stdio%20-%20detectado.png)

#### Detección del servidor MCP stdio

![Detección del MCP server stdio](docs/img/vista%20detecta%20el%20mcp%20server%20stdio.png)

#### Tools del MCP stdio (ejemplo: filesystem)

![Tools MCP stdio — filesystem](docs/img/vista%20detecta%20tools%20mcp%20stdio%20-%20filesystem.png)

#### Servidor MCP streamable (HTTP)

![Detección MCP server streamable HTTP](docs/img/vista%20detecta%20mcp%20server%20streamable%20http.png)

#### MCP streamable HTTP con Playwright

![MCP streamable HTTP — Playwright](docs/img/vista%20detecta%20mcp%20stremablehttp%20-%20playwright.png)

---

## 💡 El problema que resuelve

La mayoría de los agentes de IA saben *hablar*, pero no saben *hacer*.
Conectar un LLM a sistemas reales —un CRM, un ERP, un gestor de tareas— requiere integraciones manuales, exponer credenciales al modelo, y lidiar con flujos que se rompen a mitad del camino o entre muchos servidores MCP.

**AIgent** resuelve esto en dos capas:

### 🖐️ Capa de ejecución: HandsAI
[HandsAI](https://vrivaans.github.io/handsai-presentation/) es el puente entre el agente y el mundo real. Registrás cualquier API REST una vez, y HandsAI la expone como herramienta MCP. El agente nunca ve URLs, tokens ni credenciales — HandsAI los inyecta de forma transparente en cada llamada y protege las respuestas contra inyecciones de prompt.

> *Si AIgent es el cerebro, HandsAI son las manos.*

### 🧠 Capa de orquestación: AIgent
AIgent actúa como el cerebro agéntico que opera sobre HandsAI. No solo ejecuta herramientas: encadena operaciones complejas entre sistemas distintos (ej: Odoo → Trello → Bluesky), gestiona las API Keys y Tokens cifrados con AES-256-GCM, y nunca se detiene ante una confirmación sensible gracias al **Loop Resume** — un mecanismo que reanuda automáticamente el hilo de razonamiento del agente tras la aprobación humana.

### Los tres problemas que resuelve AIgent
1. **Seguridad**: Las credenciales nunca viajan al modelo. Ni las de las APIs externas (HandsAI) ni las de los proveedores de IA (AIgent).
2. **Resiliencia**: Los flujos multi-paso no se pierden. El agente retoma exactamente donde lo dejó tras una confirmación.
3. **Orquestación**: Un solo agente puede operar herramientas de CRM, ERP y productividad sin que el humano intervenga en cada paso.

---

## 🌟 Características principales

- **🛡️ Seguridad**: Almacenamiento de API Keys y Tokens del bridge cifrado dinámicamente con **AES-256-GCM**. Tus llaves nunca se guardan en texto plano en la base de datos ni en archivos de configuración.
- **⚙️ Configuración dinámica**: Gestión de conexiones a proveedores (Groq, OpenRouter, Gemini, etc.) y al bridge HandsAI directamente desde la UI. Los cambios se aplican en caliente sin reiniciar el servidor.
- **🔄 Resiliencia agéntica (Loop Resume)**: Tras aprobar una acción sensible, el agente reanuda automáticamente su hilo de pensamiento para completar flujos complejos (ej. Odoo → Trello) sin intervención adicional.
- **🔐 Permisos persistentes de tools**: Al aprobar una acción sensible, marcá **"Permitir siempre"** para no volver a preguntar por esa herramienta. Gestioná, pausá o revocá permisos desde la pestaña **Permisos**.
- **✅ Aprobaciones centralizadas**: La pestaña **Aprobaciones** lista confirmaciones pendientes de todas las sesiones, para aprobar o rechazar sin cambiar de chat.
- **🔌 Ecosistema de herramientas**: Integración nativa con **HandsAI** más servidores MCP stdio/stream y **skills Python** locales. Sincronización bajo demanda.
- **🌟 Agentes especializados**: Creá múltiples agentes con identidad propia, subconjunto de tools y modelo/proveedor. El agente **General** siempre tiene acceso a todas las tools del registry. Esto reduce costos de input tokens y mejora el enfoque.
- **📡 Streaming SSE**: Las respuestas del chat llegan token a token vía `POST /api/sessions/:id/chat/stream`, con logs de ejecución de tools en tiempo real y renderizado de diagramas **Mermaid** en el chat.
- **⏹️ Detener generación**: Cancelá una respuesta en curso en cualquier momento con el botón de stop.
- **✏️ Edición de prompts**: Editá un mensaje del usuario para truncar el historial desde ese punto y reintentar con un prompt corregido.
- **🔁 Fallback automático entre proveedores LLM**: Si la inferencia falla por cuota, rate limit, modelo no disponible u otros errores recuperables, el backend prueba otros proveedores activos en orden. Si el cambio tiene éxito, la sesión queda usando ese proveedor y el usuario ve un aviso en el chat.
- **🔌 MCP stdio y MCP stream (HTTP / SSE)**: Registrá servidores MCP **locales** (proceso stdin/stdout) y **remotos** (transporte HTTP streamable, típicamente SSE). Las tools se exponen con prefijo por alias y se sincronizan con el resto del catálogo.
- **📚 RAG / Base de conocimiento**: Subí documentos, fragmentalos, generá embeddings con un proveedor designado y almacená vectores en **pgvector**. Los fragmentos relevantes se inyectan automáticamente en el prompt de sistema en cada consulta.
- **⚡ Workflows RuleGo**: Creá flujos deterministas programables (JSON RuleChain) desde la UI o vía el agente. Visualizalos como diagramas **Mermaid** y ejecutalos manualmente o con cron.
- **🐍 Skills Python locales**: Colocá carpetas de skills en `skills/` con un `metadata.json` y un script — se cargan al arranque y se exponen como tools nativas.
- **🧠 Memoria e intents Invok**: Las tools core `invok_*` (guardar/buscar conocimiento, mapeo de intents) se inyectan automáticamente en todos los contextos de agente cuando HandsAI está configurado.
- **📅 Tasks programadas**: Creá tasks con cron desde el **Dashboard** o pedile al agente que las programe.
- **🌐 Interfaz bilingüe (ES / EN)**: Traducción completa de la UI mediante un servicio de traducción dinámico.
- **🎨 UX/UI**: Interfaz minimalista en **Angular 21** alineada con el visual de Invok, con estados de razonamiento, filtrado de sesiones (ocultar cron/workflows) y selección de agente/modelo por sesión.
- **⚙️ Backend de alto rendimiento**: Escrito íntegramente en **Go**, con runtime del brain modularizado (`prompt_logic`, `provider_runtime`, `tool_context`, `session_manager`).

---

## 🏗️ Decisiones de arquitectura

AIgent fue diseñado pensando en eficiencia y seguridad:

1. **¿Por qué Go?**: Baja latencia y mínima huella de memoria frente a runtimes más pesados. La mayor parte de los recursos del VPS se dedica al razonamiento del agente y al procesamiento de herramientas vía HandsAI.
2. **Seguridad proactiva (AES-256-GCM)**: Al manejar credenciales reales, implementamos cifrado simétrico dinámico. Las API Keys nunca residen en texto plano.
3. **Resiliencia en el Chain-of-Thought**: Loop Resume detecta estados de pausa y reanuda la inferencia tras la aprobación humana, garantizando que procesos complejos no se pierdan.
4. **Brain runtime modular**: El procesamiento del chat se divide en módulos de prompt, contexto de tools, runtime de proveedores y ejecución de tool-calls para reducir acoplamiento.

---

## 🔁 Resiliencia del proveedor LLM (fallback)

Durante cada llamada al modelo, AIgent construye una **lista ordenada de candidatos**:

1. **Override de la sesión** (si el usuario eligió otro proveedor/modelo para esa conversación).
2. **Proveedor del agente** activo en el chat.
3. **Proveedor marcado como default** en la pestaña de proveedores.

El primero que aplique es el **preferido**; el resto de proveedores **activos** se añaden como respaldo (priorizando el que también esté marcado como default entre los secundarios).

Si la API del preferido devuelve un error **recuperable** (cuota insuficiente, rate limit `429`, modelo no encontrado, clave inválida, `401`/`403`, etc.), el sistema **reintenta la misma petición** con el siguiente candidato. Cuando un fallback **funciona**:

- Se **persiste** en la base de datos un override de proveedor para esa sesión (y se limpia el override de modelo si había uno).
- El frontend puede mostrar un mensaje *provider_fallback* indicando el cambio (proveedor y modelo anteriores → nuevos).

Si el error **no** es recuperable, no hay cadena de fallback. El fallback también aplica a peticiones en **streaming**.

---

## 🔌 Servidores MCP además de HandsAI

HandsAI sigue siendo la capa principal para APIs REST registradas, pero AIgent integra también **Model Context Protocol** de dos formas:

### MCP stdio (proceso local)

- Configurás un **comando**, **argumentos** y **variables de entorno** (los secretos sensibles se guardan cifrados en base de datos).
- El servidor arranca como subproceso y habla MCP por **stdin/stdout**.
- Rutas bajo `/api/config/mcp-stdio` (listar, crear, editar, borrar y **probar conexión**).

### MCP stream / HTTP (remoto, SSE)

- Configurás una **URL base** y **cabeceras HTTP** opcionales (campos sensibles cifrados).
- El cliente usa el transporte **HTTP streamable** habitual de MCP (muchas implementaciones usan **SSE**).
- Opción `disable_standalone_sse` para entornos donde el servidor no expone SSE standalone.
- Rutas API: `/api/config/mcp-stream` con las mismas operaciones CRUD y test que stdio.

Tras guardar o actualizar una entrada, el backend **recarga integraciones** y **vuelve a sincronizar** el registro de herramientas. Las tools de MCP aparecen con un **prefijo por alias** (p. ej. `mi_servidor_nombre_tool`).

---

## 📚 Base de conocimiento (RAG)

AIgent incluye una capa de recuperación basada en **pgvector**:

1. **Subí documentos** vía `POST /api/rag/upload` (PDF, TXT, MD, HTML, etc.) con tamaño de chunk y solapamiento configurables.
2. **Designá un proveedor de embeddings** en la pestaña de Proveedores LLM (checkbox **Proveedor de Embeddings**). Debe haber al menos un proveedor activo marcado para embeddings.
3. Los fragmentos se embeben y almacenan en PostgreSQL con búsqueda por similitud vectorial.
4. En cada consulta del usuario, el backend recupera los fragmentos más relevantes y los inyecta en el prompt de sistema bajo `=== CONTEXTO DE CONOCIMIENTO RELEVANTE (RAG) ===`.
5. Búsqueda manual vía `GET/POST /api/rag/search`.

El agente está instruido para responder directamente desde el contexto RAG inyectado en lugar de llamar herramientas de búsqueda de forma redundante.

---

## ⚡ Workflows deterministas (RuleGo)

Además de la orquestación agéntica libre, AIgent integra el motor **RuleGo** para flujos repetibles y programables:

- Creá workflows desde la pestaña **Workflows** o pedile al agente que construya uno con la tool `save_workflow`.
- Cada workflow es una definición JSON **RuleChain**, visualizada como diagrama **Mermaid** en la UI.
- Ejecutalos manualmente o con **cron**; el historial de ejecución se registra por run.
- Las sesiones generadas por workflows pueden ocultarse del sidebar (filtrado de sesiones).

El agente puede llamar a `get_workflow_guide` para conocer el esquema RuleChain y los nodos de tools disponibles.

---

## 🐍 Skills Python locales

Colocá carpetas de skills bajo `skills/`, cada una con:

```
skills/
  mi_skill/
    metadata.json   # nombre, descripción, esquema de parámetros, script, flag sensitive
    mi_skill.py     # script ejecutable
```

Las skills se escanean al arranque y se registran en el catálogo de tools. Ejemplo incluido: `skills/ping_host/`.

---

## ⚡ Smart Context Cache (SCC) — Experimental

> **Estado: implementado, aún sin testear completamente.** La funcionalidad está disponible en la UI y el backend, pero falta validación end-to-end (cache hits entre proveedores, ganancias de costo/latencia, casos límite).

Smart Context Cache organiza cada petición al LLM en tres capas de volatilidad para maximizar el **context caching** entre proveedores (DeepSeek, Anthropic, Gemini, OpenAI, etc.):

| Capa | Contenido | Volatilidad |
|------|-----------|-------------|
| **Capa 1** | Prompt de sistema, contratos de tools, especificación RuleGo | Inmutable (0%) |
| **Capa 2** | Objetivos de sesión, archivos del workspace local, archivos subidos a la sesión | Semi-estática (baja) |
| **Capa 3** | Historial de chat y mensaje actual del usuario | Dinámica (100%) |

### Qué podés configurar hoy (Capa 2)

Desde el panel del chat (toggle ⚡):

- **Objetivos de sesión** — instrucciones de foco para la conversación actual (ej. *"Hoy solo refinamos los tests unitarios"*).
- **Workspace local** — ruta a un directorio de proyecto, con explorador de carpetas. Los comandos se ejecutan dentro del límite del workspace (sandboxing por rutas canónicas).
- **Archivos de sesión** — subí PDF, HTML, TXT, MD, JSON, CSV o XLSX adjuntos al contexto de la sesión.

Endpoints API: `POST /api/sessions/:id/goals`, `POST /api/sessions/:id/workspace`, `POST/GET/DELETE /api/sessions/:id/files`, `GET /api/workspace/browse`.

Ver `docs/smart-context-cache-specification.md` para el diseño técnico completo (adaptadores de caché por proveedor, hashing determinista, compactación de cola, etc.).

---

## 🛠️ Stack tecnológico

- **Frontend**: Angular 21 (Signals, Standalone Components, CSS vanilla).
- **Backend**: Go 1.25+ (Fiber, GORM).
- **Base de datos**: PostgreSQL con **pgvector** (`pgvector/pgvector:pg15`).
- **IA**: Orquestación agéntica mediante OpenRouter / Groq / Gemini y otros proveedores compatibles con OpenAI.
- **Workflows**: Motor RuleGo con visualización Mermaid.
- **RAG**: Parsing de documentos con LangChainGo + búsqueda por similitud en pgvector.
- **Infraestructura**: Docker & Docker Compose (listo para **CubePath**).

---

## 🚀 Instalación y despliegue

### Requisitos previos
- Docker y Docker Compose instalados.
- Un navegador moderno.

### Pasos para el despliegue

1. **Configuración**: Copiá el archivo de ejemplo y configurá tu `DB_ENCRYPTION_KEY` (cadena de 32 caracteres aleatorios) más `ADMIN_USERNAME` y `ADMIN_PASSWORD`.
   ```bash
   cp .env.example .env
   ```
2. **Levantar el sistema**: Docker Compose construye y ejecuta la app (API + Angular estático) y la base de datos.
   ```bash
   docker-compose up -d --build
   ```
3. **Acceso**:
   - **Producción (Docker)**: `http://localhost:3000` (API y UI en el mismo puerto)
   - **Desarrollo local**: servidor de desarrollo Angular en `http://localhost:4200` (proxy al API Go en `:3000`)

---

## 📖 Cómo funciona

1. **Configurá tu cerebro**: En **Proveedores de LLM**, añadí uno o más proveedores para redundancia. Marcá uno como **default** y opcionalmente uno como **Proveedor de Embeddings** para RAG. Usá el dropdown de modelos con refresco para elegirlos dinámicamente. Las llaves se cifran tras un test de conexión exitoso.
2. **Conectá tus manos**: Configurá la URL y el Token del bridge **HandsAI**. Opcionalmente, registrá servidores **MCP stdio** y **MCP stream** adicionales.
3. **Establecé reglas**: Definí reglas como *"Sé siempre conciso"* o *"Valida el ID de Odoo antes de crear nada"*.
4. **Agentes y herramientas**: En **Agentes**, definí modelo/proveedor y subconjunto de tools por personalidad. Cambiá de agente por sesión en el chat. Reseteá overrides de LLM si hubo fallback o cambio manual.
5. **Subí conocimiento (RAG)**: Cargá documentos para que el agente responda desde tus datos. Los fragmentos relevantes se inyectan automáticamente en cada mensaje.
6. **Smart Context Cache (opcional)**: Configurá objetivos de sesión, apuntá a un workspace local y adjuntá archivos para trabajo de contexto extenso. *Aún pendiente de testeo completo.*
7. **Automatizá**: Pedí tareas complejas — *"Creá una tarea en Trello y registrála en Odoo"* — o creá **tasks programadas** y **workflows RuleGo** desde el Dashboard / pestaña Workflows.
8. **Aprobá y gestioná permisos**: Las acciones sensibles pausan para confirmación. Marcá **Permitir siempre** para persistir el permiso, o administrá todo desde la pestaña **Permisos**. Las acciones pendientes de todas las sesiones aparecen en **Aprobaciones**.

---

## ⚠️ Nota sobre la elección del modelo

> El rendimiento de AIgent depende directamente de las capacidades de razonamiento del modelo configurado. Los modelos **menores de 100B parámetros** funcionan bien para tareas simples y consultas directas, pero pueden tener dificultades para encadenar flujos de ejecución complejos entre múltiples herramientas. Para aprovechar al máximo la orquestación de AIgent se recomienda usar modelos **de 100B parámetros o más**.
