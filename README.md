# AIgent: El Orquestador de Agentes Digitales 🤖🚀

[![Hackaton CubePath 2026](https://img.shields.io/badge/Hackaton-CubePath_2026-blueviolet?style=for-the-badge)](https://github.com/midudev/hackaton-cubepath-2026)
[![Powered by Go](https://img.shields.io/badge/Powered_by-Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Angular 21](https://img.shields.io/badge/Angular-21-DD0031?style=for-the-badge&logo=angular&logoColor=white)](https://angular.io/)

**AIgent** es un operador  diseñado para actuar como un puente seguro y resiliente entre el usuario y sus herramientas de trabajo (operando las tools de HandsAI principalmente). A diferencia de los chatbots tradicionales, AIgent no solo habla: **ejecuta**. El propósito es que el agente pueda operar herramientas de forma autónoma y segura, y que esas herramientas sean software de terceros.

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

## 💡 El problema que resuelve

La mayoría de los agentes de IA saben *hablar*, pero no saben *hacer*.
Conectar un LLM a sistemas reales —un CRM, un ERP, un gestor de tareas— requiere
integraciones manuales, exponer credenciales al modelo, y lidiar con flujos que se
rompen cuando algo falla a mitad del camino o muchos MCP servers.

**AIgent** resuelve esto en dos capas:

### 🖐️ Capa de ejecución: HandsAI
[HandsAI](https://vrivaans.github.io/handsai-presentation/) es el puente entre el
agente y el mundo real. Registrás cualquier API REST una vez, y HandsAI la expone
como herramienta MCP. El agente nunca ve URLs, tokens ni credenciales — HandsAI
los inyecta de forma transparente en cada llamada y protege las respuestas contra
inyecciones de prompt.

> *Si AIgent es el cerebro, HandsAI son las manos.*

### 🧠 Capa de orquestación: AIgent
AIgent actúa como el cerebro agéntico que opera sobre HandsAI. No solo ejecuta
herramientas: encadena operaciones complejas entre sistemas distintos
(ej: Odoo → Trello → Bluesky), gestiona las API Keys y Tokens de forma
cifrada con AES-256-GCM, y nunca se detiene ante una confirmación sensible gracias
al **Loop Resume** — un mecanismo que reanuda automáticamente el hilo de
razonamiento del agente tras la aprobación humana.

### Los tres problemas que resuelve AIgent
1. **Seguridad**: Las credenciales nunca viajan al modelo. Ni las de las APIs externas
   (HandsAI) ni las de los proveedores de IA (AIgent).
2. **Resiliencia**: Los flujos multi-paso no se pierden. El agente retoma exactamente
   donde lo dejó tras una confirmación.
3. **Orquestación**: Un solo agente puede operar herramientas de CRM, ERP y
   productividad sin que el humano intervenga en cada paso.

---

## 🌟 Características Principales

- **🛡️ Seguridad**: Almacenamiento de API Keys y Tokens del Bridge cifrado dinámicamente con **AES-256-GCM**. Tus llaves nunca se guardan en texto plano en la base de datos ni en archivos de configuración.
- **⚙️ Configuración Dinámica**: Gestión de conexiones a proveedores (Groq, OpenRouter) y puentes (HandsAI) directamente desde la UI. Los cambios se aplican en caliente sin reiniciar el servidor.
- **🔄 Resiliencia Agéntica (Loop Resume)**: El sistema nunca se detiene. Tras una confirmación de acción sensible, el agente reanuda automáticamente su hilo de pensamiento para completar flujos complejos (ej. Odoo -> Trello) sin intervención adicional.
- **🔌 Ecosistema de Herramientas**: Integración nativa con **HandsAI** para ejecutar herramientas, permitiendo automatizar flujos reales de negocio con sincronización bajo demanda.
- **🌟 Agentes Especializados**: Ya no dependés de un único bot monolítico. Podés crear múltiples Agentes con identidades propias, eligiendo qué herramientas exactas pueden acceder y con qué modelo o proveedor (Groq, OpenRouter) procesarán la información. Esto salva masivamente los costos limitando el uso de `Input Tokens` y mejora el enfoque (reduciendo alucinaciones).
- **🎨 UX/UI**: Interfaz minimalista en **Angular 21** con visualización del flujo de pensamiento (logs de ejecución) y estados de razonamiento en tiempo real.
- **⚙️ Backend de Alto Rendimiento**: Escrito íntegramente en **Go**, garantizando concurrencia, velocidad y bajo consumo de recursos.
- **🔁 Fallback automático entre proveedores LLM**: Si la inferencia falla por cuota, rate limit, modelo no disponible u otros errores recuperables, el backend prueba **otros proveedores activos** en orden hasta que uno responda. Si el cambio tiene éxito, la sesión queda usando ese proveedor y el usuario ve un aviso en el chat.
- **🔌 MCP stdio y MCP stream (HTTP / SSE)**: Además de **HandsAI**, podés registrar servidores MCP **locales** (proceso por stdin/stdout) y **remotos** (URL HTTP con transporte streamable, típicamente SSE). Las herramientas se exponen al agente con un prefijo por alias y se sincronizan junto al resto del catálogo.

## 🏗️ Decisiones de Arquitectura

En una competencia donde cada byte cuenta, AIgent ha sido diseñado pensando en la eficiencia y la seguridad:

1.  **¿Por qué Go?**: Se eligió Go por su baja latencia y su mínima huella de memoria en comparación con otros lenguajes como Java. Esto permite que el **90% de los recursos del VPS** se dediquen exclusivamente al razonamiento del agente y al procesamiento pesado de herramientas mediante HandsAI.
2.  **Seguridad Proactiva (AES-256-GCM)**: Dado que manejamos identidades y credenciales reales, implementamos cifrado simétrico dinámico. Las API Keys nunca residen en texto plano, ni siquiera en variables de entorno fijas después de su configuración inicial.
3.  **Resiliencia en el Chain-of-Thought**: Implementamos una lógica de "Loop Resume" que detecta estados de pausa y reanuda la inferencia tras la aprobación humana. Esto garantiza que procesos complejos (ej: "Crear en Odoo -> Crear en Trello") no se pierdan en el tiempo.

---

## 🔁 Resiliencia del proveedor LLM (fallback)

Durante cada llamada al modelo, AIgent construye una **lista ordenada de candidatos**:

1. **Override de la sesión** (si el usuario eligió otro proveedor/modelo para esa conversación).
2. **Proveedor del agente** activo en el chat.
3. **Proveedor marcado como default** en la pestaña de proveedores.

El primero que aplique es el **preferido**; el resto de proveedores **activos** se añaden como respaldo (priorizando el que también esté marcado como default entre los secundarios).

Si la API del preferido devuelve un error considerado **recuperable** (por ejemplo: cuota insuficiente, rate limit `429`, modelo no encontrado, clave inválida, `401`/`403`, etc.), el sistema **reintenta la misma petición** con el siguiente candidato, y así sucesivamente. Cuando un fallback **funciona**:

- Se **persiste** en la base de datos un override de proveedor para esa sesión (y se limpia el override de modelo si había uno), de modo que los siguientes mensajes sigan usando el proveedor que respondió bien.
- El frontend puede mostrar un mensaje del tipo *provider_fallback* indicando el cambio (proveedor y modelo anteriores → nuevos).

Si el error **no** se considera recuperable, no hay cadena de fallback: se devuelve el error al usuario. Así se evita enmascarar fallos de validación o de red que no tienen sentido “saltar” a otro LLM.

---

## 🔌 Servidores MCP además de HandsAI

HandsAI sigue siendo la capa principal para APIs REST registradas, pero AIgent integra también **Model Context Protocol** de dos formas:

### MCP stdio (proceso local)

- Configurás un **comando**, **argumentos** y **variables de entorno** (los secretos sensibles se guardan cifrados en base de datos).
- El servidor arranca como subproceso y habla MCP por **stdin/stdout**.
- Desde la UI/API: rutas bajo `/api/config/mcp-stdio` (listar, crear, editar, borrar y **probar conexión**).

### MCP stream / HTTP (remoto, SSE)

- Configurás una **URL base** y **cabeceras HTTP** opcionales (también con campos sensibles cifrados).
- El cliente usa el transporte **HTTP streamable** habitual de MCP (muchas implementaciones usan **SSE**).
- Opción `disable_standalone_sse` para entornos donde el servidor no expone SSE “standalone” y hay que ajustar el comportamiento del cliente.
- Rutas API: `/api/config/mcp-stream` con las mismas operaciones CRUD y test que stdio.

En ambos casos, tras guardar o actualizar una entrada, el backend **recarga integraciones** y **vuelve a sincronizar** el registro de herramientas para que el agente vea los nombres y esquemas actualizados. Las tools de MCP suelen aparecer con un **prefijo por alias** (p. ej. `mi_servidor_nombre_tool`) para no chocar con HandsAI ni entre servidores.

---

## 🛠️ Stack Tecnológico

- **Frontend**: Angular 21 (Signals, Standalone Components, CSS Vanilla).
- **Backend**: Go 1.22+ (Fiber, GORM).
- **Base de Datos**: PostgreSQL (Almacenamiento persistente de sesiones y reglas).
- **IA**: Orquestación agéntica mediante OpenRouter / Groq / Gemini.
- **Infraestructura**: Docker & Docker Compose (Listo para **CubePath**).

---

## 🚀 Instalación y Despliegue

### Requisitos Previos
- Docker y Docker Compose instalados.
- Un navegador moderno.

### Pasos para el Despliegue
1. **Configuración**: Copia el archivo de ejemplo y configura tu `DB_ENCRYPTION_KEY` (una cadena de 32 caracteres aleatorios).
   ```bash
   cp .env.example .env
   ```
2. **Levantar el Sistema**: Usa docker-compose para levantar el Backend, el Frontend y la Base de Datos.
   ```bash
   docker-compose up -d --build
   ```
3. **Acceso**:
   - Frontend: `http://localhost:4200`
   - API: `http://localhost:8080`

---

## 📖 Cómo Funciona

1. **Configura tu Cerebro**: Ve a la pestaña "Proveedores de LLM" y añade **varios** proveedores si querés redundancia: el primero que corresponda al agente o al default se usa, y el resto actúa como **fallback** automático si el primero falla por cuota, modelo no disponible, etc. AIgent probará la conexión y guardará las llaves cifradas.
2. **Conecta tus Manos**: En la misma sección, configura la URL y el Token de tu bridge **HandsAI**. Opcionalmente, en **MCP stdio** y **MCP stream**, registrá servidores adicionales (CLI locales o endpoints remotos) para ampliar el catálogo de herramientas.
3. **Establece Reglas**: AIgent aprende cómo trabajar. Puedes definir reglas como *"Sé siempre conciso"* o *"Valida el ID de Odoo antes de crear nada"*.
4. **Agentes y herramientas**: En "Agentes" definís qué modelo/proveedor usa cada personalidad y qué subconjunto de tools puede invocar; en el chat podés **volver al default del agente** si aplicaste un override o un fallback.
5. **Automatiza**: Pide cosas complejas: *"Crea una tarea en Trello en el tablero de Hackatón, y luego regístrala también en el CRM de Odoo"*. Observa cómo AIgent encadena las herramientas, te pide confirmación solo para lo más crítico y sincroniza las capacidades en tiempo real.

---

## ⚠️ Nota sobre la elección del modelo
> El rendimiento de AIgent depende directamente de las capacidades de razonamiento
> del modelo configurado. Los modelos **menores de 100B parámetros** funcionan bien
> para tareas simples y consultas directas, pero pueden tener dificultades para
> encadenar flujos de ejecución complejos entre múltiples herramientas.
> Para aprovechar al máximo la orquestación de AIgent se recomienda usar modelos
> **de 100B parámetros o más**
