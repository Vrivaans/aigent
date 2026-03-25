# AIgent: El Orquestador de Agentes Digitales 🤖🚀

[![Hackaton CubePath 2026](https://img.shields.io/badge/Hackaton-CubePath_2026-blueviolet?style=for-the-badge)](https://github.com/midudev/hackaton-cubepath-2026)
[![Powered by Go](https://img.shields.io/badge/Powered_by-Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Angular 21](https://img.shields.io/badge/Angular-21-DD0031?style=for-the-badge&logo=angular&logoColor=white)](https://angular.io/)

**AIgent** es un potente orquestador de agentes de IA diseñado para actuar como un puente seguro y resiliente entre el usuario y sus herramientas de trabajo (Trello, Odoo, HandsAI). A diferencia de los chatbots tradicionales, AIgent no solo habla: **ejecuta**.

---

## 💡 ¿Por qué AIgent?

En el ecosistema actual de IA, la mayoría de los bots se limitan a responder preguntas o generar texto. El verdadero reto surge cuando queremos que la IA **opere** herramientas reales de forma segura. 

AIgent nace para resolver tres problemas críticos en la automatización con agentes:
1. **Seguridad**: Gestionar API Keys de forma centralizada y cifrada para el uso de modelos.
2. **Resiliencia**: Que el agente no se detenga si una tarea requiere múltiples pasos y confirmaciones sensibles.
3. **Orquestación**: Unificar herramientas dispares (CRM, Productividad, ERP) bajo un mismo cerebro agéntico.

---

## 🌟 Características Principales

- **🛡️ Seguridad Grado Industrial**: Almacenamiento de API Keys cifrado dinámicamente con **AES-256-GCM**. Tus llaves nunca se guardan en texto plano en la base de datos ni en archivos de configuración.
- **🔄 Resiliencia Agéntica (Loop Resume)**: El sistema nunca se detiene. Tras una confirmación de acción sensible, el agente reanuda automáticamente su hilo de pensamiento para completar flujos complejos (ej. Odoo -> Trello) sin intervención adicional.
- **🔌 Ecosistema de Herramientas**: Integración nativa con **HandsAI** para ejecutar herramientas MCP, permitiendo automatizar flujos reales de negocio.
- **🎨 Experiencia Premium**: Interfaz minimalista en **Angular 21** con visualización del flujo de pensamiento (logs de ejecución) y estados de razonamiento en tiempo real.
- **⚙️ Backend de Alto Rendimiento**: Escrito íntegramente en **Go**, garantizando concurrencia, velocidad y bajo consumo de recursos.

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

---

Desarrollado con ❤️ para la Hackatón CubePath 2026.
