# AIgent: El Orquestador de Agentes Digitales 🤖🚀

[![Hackaton CubePath 2026](https://img.shields.io/badge/Hackaton-CubePath_2026-blueviolet?style=for-the-badge)](https://github.com/midudev/hackaton-cubepath-2026)
[![Powered by Go](https://img.shields.io/badge/Powered_by-Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Angular 21](https://img.shields.io/badge/Angular-21-DD0031?style=for-the-badge&logo=angular&logoColor=white)](https://angular.io/)

**AIgent** es un operador  diseñado para actuar como un puente seguro y resiliente entre el usuario y sus herramientas de trabajo (operando las tools de HandsAI principalmente). A diferencia de los chatbots tradicionales, AIgent no solo habla: **ejecuta**. El propósito es que el agente pueda operar herramientas de forma autónoma y segura, y que esas herramientas sean software de terceros.

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
(ej: Odoo → Trello → Bluesky), gestiona las API Keys de los modelos de forma
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

- **🛡️ Seguridad**: Almacenamiento de API Keys cifrado dinámicamente con **AES-256-GCM**. Tus llaves nunca se guardan en texto plano en la base de datos ni en archivos de configuración.
- **🔄 Resiliencia Agéntica (Loop Resume)**: El sistema nunca se detiene. Tras una confirmación de acción sensible, el agente reanuda automáticamente su hilo de pensamiento para completar flujos complejos (ej. Odoo -> Trello) sin intervención adicional.
- **🔌 Ecosistema de Herramientas**: Integración nativa con **HandsAI** para ejecutar herramientas MCP, permitiendo automatizar flujos reales de negocio.
- **🎨 UX/UI**: Interfaz minimalista en **Angular 21** con visualización del flujo de pensamiento (logs de ejecución) y estados de razonamiento en tiempo real.
- **⚙️ Backend de Alto Rendimiento**: Escrito íntegramente en **Go**, garantizando concurrencia, velocidad y bajo consumo de recursos.

## 🏗️ Decisiones de Arquitectura

En una competencia donde cada byte cuenta, AIgent ha sido diseñado pensando en la eficiencia y la seguridad:

1.  **¿Por qué Go?**: Se eligió Go por su baja latencia y su mínima huella de memoria en comparación con otros lenguajes como Java. Esto permite que el **90% de los recursos del VPS** se dediquen exclusivamente al razonamiento del agente y al procesamiento pesado de herramientas mediante HandsAI.
2.  **Seguridad Proactiva (AES-256-GCM)**: Dado que manejamos identidades y credenciales reales, implementamos cifrado simétrico dinámico. Las API Keys nunca residen en texto plano, ni siquiera en variables de entorno fijas después de su configuración inicial.
3.  **Resiliencia en el Chain-of-Thought**: Implementamos una lógica de "Loop Resume" que detecta estados de pausa y reanuda la inferencia tras la aprobación humana. Esto garantiza que procesos complejos (ej: "Crear en Odoo -> Crear en Trello") no se pierdan en el tiempo.

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

1. **Configura tu Cerebro**: Ve a la pestaña "Modelos IA" y añade tu proveedor favorito (Groq, Gemini, etc.). AIgent probará la conexión y guardará la llave de forma cifrada.
2. **Establece Reglas**: AIgent aprende cómo trabajar. Puedes definir reglas como *"Sé siempre conciso"* o *"Valida el ID de Odoo antes de crear nada"*.
3. **Automatiza**: Pide cosas complejas: *"Crea una tarea en Trello en el tablero de Hackatón, y luego regístrala también en el CRM de Odoo"*. Observa cómo AIgent encadena las herramientas y te pide confirmación solo para lo más crítico.

---

## 📽️ Demo y Video
*(Enlaza aquí tu video de presentación de YouTube/Loom)*
