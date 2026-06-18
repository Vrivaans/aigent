# AIgent: The Digital Agent Orchestrator 🤖🚀

[![Hackaton CubePath 2026](https://img.shields.io/badge/Hackaton-CubePath_2026-blueviolet?style=for-the-badge)](https://github.com/midudev/hackaton-cubepath-2026)
[![Powered by Go](https://img.shields.io/badge/Powered_by-Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Angular 21](https://img.shields.io/badge/Angular-21-DD0031?style=for-the-badge&logo=angular&logoColor=white)](https://angular.io/)

**English** · [Español](README-es.md)

**AIgent** is an operator designed to act as a secure, resilient bridge between the user and their work tools (primarily HandsAI tools). Unlike traditional chatbots, AIgent does not just talk — it **executes**. The goal is for the agent to operate third-party software autonomously and safely.

---

## 🎬 Demo & Screenshots

| | |
|:--|:--|
| **Video (YouTube)** | [Live demo — real-time tool execution](https://youtu.be/N7zXwUHNL5k) |

### Main UI & Flow

#### Chat: the agent executes tools in real time (filesystem)

![Chat: the agent can operate the filesystem](docs/img/vista%20chat%20puede%20operar%20filesystem.png)

#### Specialized agents

![Agents view](docs/img/vista%20de%20agentes.png)

#### Rules for agent behavior

![Rules view](docs/img/vista%20de%20reglas%20para%20agentes.png)

#### LLM providers

![LLM providers view](docs/img/vista%20de%20proveedores%20llm.png)

#### Tool catalog

![Tool catalog view](docs/img/vista%20de%20tools.png)

### MCP Integration (stdio & HTTP streamable)

#### New MCP stdio server registered and detected

![New MCP stdio server detected](docs/img/vista%20nuevo%20servidor%20mcp%20stdio%20-%20detectado.png)

#### MCP stdio server detection

![MCP stdio server detection](docs/img/vista%20detecta%20el%20mcp%20server%20stdio.png)

#### MCP stdio tools (example: filesystem)

![MCP stdio tools — filesystem](docs/img/vista%20detecta%20tools%20mcp%20stdio%20-%20filesystem.png)

#### MCP streamable server (HTTP)

![MCP streamable HTTP server detection](docs/img/vista%20detecta%20mcp%20server%20streamable%20http.png)

#### MCP streamable HTTP with Playwright

![MCP streamable HTTP — Playwright](docs/img/vista%20detecta%20mcp%20stremablehttp%20-%20playwright.png)

---

## 💡 The Problem It Solves

Most AI agents know how to *talk*, but not how to *act*.
Connecting an LLM to real systems — a CRM, an ERP, a task manager — requires manual integrations, exposing credentials to the model, and dealing with flows that break halfway through or across many MCP servers.

**AIgent** solves this in two layers:

### 🖐️ Execution layer: HandsAI
[HandsAI](https://vrivaans.github.io/handsai-presentation/) is the bridge between the agent and the real world. Register any REST API once, and HandsAI exposes it as an MCP tool. The agent never sees URLs, tokens, or credentials — HandsAI injects them transparently on every call and protects responses against prompt injection.

> *If AIgent is the brain, HandsAI is the hands.*

### 🧠 Orchestration layer: AIgent
AIgent acts as the agentic brain operating on top of HandsAI. It does not just execute tools: it chains complex operations across different systems (e.g. Odoo → Trello → Bluesky), manages API keys and tokens encrypted with AES-256-GCM, and never stops at a sensitive confirmation thanks to **Loop Resume** — a mechanism that automatically resumes the agent's reasoning thread after human approval.

### The three problems AIgent solves
1. **Security**: Credentials never reach the model — neither external API credentials (HandsAI) nor AI provider keys (AIgent).
2. **Resilience**: Multi-step flows are not lost. The agent picks up exactly where it left off after a confirmation.
3. **Orchestration**: A single agent can operate CRM, ERP, and productivity tools without the human intervening at every step.

---

## 🌟 Key Features

- **🛡️ Security**: API keys and bridge tokens are stored encrypted with **AES-256-GCM**. Keys are never saved in plain text in the database or config files.
- **⚙️ Dynamic configuration**: Manage provider connections (Groq, OpenRouter, Gemini, etc.) and the HandsAI bridge directly from the UI. Changes apply hot without restarting the server.
- **🔄 Agentic resilience (Loop Resume)**: After approving a sensitive action, the agent automatically resumes its reasoning thread to complete complex flows (e.g. Odoo → Trello) without extra intervention.
- **🔐 Persistent tool permissions**: When approving a sensitive action, check **"Always allow"** to skip future prompts for that tool. Manage, pause, or revoke permissions from the dedicated **Permissions** tab.
- **✅ Centralized approvals**: The **Approvals** tab lists pending confirmations across all sessions, so you can approve or reject actions without switching chats.
- **🔌 Tool ecosystem**: Native **HandsAI** integration plus MCP stdio/stream servers and local **Python skills**. Tools sync on demand.
- **🌟 Specialized agents**: Create multiple agents with their own identity, tool subset, and model/provider. The **General** agent always has access to the full tool registry. This reduces input token costs and improves focus.
- **📡 SSE streaming**: Chat responses stream token by token via `POST /api/sessions/:id/chat/stream`, with real-time tool execution logs and **Mermaid** diagram rendering in the chat.
- **⏹️ Stop generation**: Cancel an in-progress response at any time with the stop button.
- **✏️ Prompt editing**: Edit a user message to truncate history from that point and retry with a revised prompt.
- **🔁 Automatic LLM fallback**: If inference fails due to quota, rate limit, unavailable model, or other recoverable errors, the backend tries other active providers in order. On success, the session switches to the working provider and the user sees a notice in chat.
- **🔌 MCP stdio & MCP stream (HTTP / SSE)**: Register **local** MCP servers (stdin/stdout process) and **remote** ones (HTTP streamable transport, typically SSE). Tools are prefixed by alias and synced with the rest of the catalog.
- **📚 RAG / Knowledge base**: Upload documents, chunk them, generate embeddings with a designated provider, and store vectors in **pgvector**. Relevant chunks are automatically injected into the system prompt on each query.
- **⚡ RuleGo workflows**: Create deterministic, schedulable workflows (RuleChain JSON) from the UI or via the agent. Visualize them as **Mermaid** diagrams and run them manually or on a cron schedule.
- **🐍 Local Python skills**: Drop skill folders under `skills/` with a `metadata.json` and a script — they are loaded at startup and exposed as native tools.
- **🧠 Invok memory & intents**: Core `invok_*` tools (knowledge save/search, intent mapping) are automatically injected into every agent context when HandsAI is configured.
- **📅 Scheduled tasks**: Create cron tasks from the **Dashboard** UI or ask the agent to schedule them.
- **🌐 Bilingual UI (EN / ES)**: Full interface translation via a dynamic translation service.
- **🎨 UX/UI**: Minimalist **Angular 21** interface aligned with the Invok visual system, with reasoning states, session filtering (hide cron/workflow sessions), and per-session agent/model selection.
- **⚙️ High-performance backend**: Written entirely in **Go**, with a modular brain runtime (`prompt_logic`, `provider_runtime`, `tool_context`, `session_manager`).

---

## 🏗️ Architecture Decisions

AIgent was designed for efficiency and security:

1. **Why Go?**: Low latency and a small memory footprint compared to heavier runtimes. Most VPS resources go to agent reasoning and tool processing via HandsAI.
2. **Proactive security (AES-256-GCM)**: Since we handle real credentials, we use dynamic symmetric encryption. API keys never live in plain text, even in fixed environment variables after initial setup. Session JWTs are signed with a separate `JWT_SECRET` (see `docs/corporate-roadmap/SECRETS.md`).

### Corporate roadmap (complete)

Phases 1–7 of the enterprise roadmap are implemented: RBAC, audit trail, approvals, secrets separation, Smart Context Cache, MCP catalog, and multi-tenant data isolation (`tenant_id` on core tables, JWT claims, scoped queries). See [docs/corporate-roadmap/ROADMAP.md](docs/corporate-roadmap/ROADMAP.md) and [docs/corporate-roadmap/STATE.json](docs/corporate-roadmap/STATE.json).
3. **Chain-of-thought resilience**: Loop Resume detects pause states and resumes inference after human approval, so complex processes (e.g. "Create in Odoo → Create in Trello") are not lost.
4. **Modular brain runtime**: Chat processing is split into prompt building, tool context, provider runtime, and tool execution modules to reduce coupling and ease evolution.

---

## 🔁 LLM Provider Resilience (Fallback)

On each model call, AIgent builds an **ordered candidate list**:

1. **Session override** (if the user chose another provider/model for that conversation).
2. **Active agent's provider**.
3. **Provider marked as default** in the providers tab.

The first applicable entry is the **preferred** one; remaining **active** providers are added as backups (prioritizing the one also marked as default among secondaries).

If the preferred API returns a **recoverable** error (insufficient quota, rate limit `429`, model not found, invalid key, `401`/`403`, etc.), the system **retries the same request** with the next candidate. When a fallback **succeeds**:

- A provider override is **persisted** in the database for that session (and any model override is cleared), so subsequent messages keep using the working provider.
- The frontend may show a *provider_fallback* message indicating the switch (previous provider/model → new ones).

Non-recoverable errors are returned directly — validation or network failures are not masked by switching LLMs. Fallback also applies to **streaming** requests.

---

## 🔌 MCP Servers Beyond HandsAI

HandsAI remains the main layer for registered REST APIs, but AIgent also integrates **Model Context Protocol** in two ways:

### MCP stdio (local process)

- Configure a **command**, **arguments**, and **environment variables** (sensitive secrets are stored encrypted in the database).
- The server starts as a subprocess and speaks MCP over **stdin/stdout**.
- UI/API routes under `/api/config/mcp-stdio` (list, create, edit, delete, and **test connection**).

### MCP stream / HTTP (remote, SSE)

- Configure a **base URL** and optional **HTTP headers** (sensitive fields encrypted).
- The client uses the standard MCP **HTTP streamable** transport (many implementations use **SSE**).
- `disable_standalone_sse` option for environments where the server does not expose standalone SSE.
- API routes: `/api/config/mcp-stream` with the same CRUD and test operations as stdio.

After saving or updating an entry, the backend **reloads integrations** and **re-syncs** the tool registry. MCP tools appear with an **alias prefix** (e.g. `my_server_tool_name`) to avoid collisions with HandsAI or other servers.

---

## 📚 Knowledge Base (RAG)

AIgent includes a built-in retrieval layer powered by **pgvector**:

1. **Upload documents** via `POST /api/rag/upload` (PDF, TXT, MD, HTML, etc.) with configurable chunk size and overlap.
2. **Designate an embeddings provider** in the LLM Providers tab (`Embeddings Provider` checkbox). At least one active provider must be marked for embeddings.
3. Chunks are embedded and stored in PostgreSQL with vector similarity search.
4. On each user query, the backend retrieves the most relevant chunks and injects them into the system prompt under `=== RELEVANT KNOWLEDGE CONTEXT (RAG) ===`.
5. Search manually via `GET/POST /api/rag/search`.

The agent is instructed to answer directly from injected RAG context rather than calling search tools redundantly.

---

## ⚡ Deterministic Workflows (RuleGo)

Beyond free-form agentic orchestration, AIgent integrates the **RuleGo** engine for repeatable, schedulable flows:

- Create workflows from the **Workflows** tab or ask the agent to build one with the `save_workflow` tool.
- Each workflow is a **RuleChain** JSON definition, visualized as a **Mermaid** diagram in the UI.
- Run workflows manually or on a **cron** schedule; execution history is tracked per run.
- Workflow-triggered chat sessions can be hidden from the sidebar (session filtering).

The agent can call `get_workflow_guide` to learn the RuleChain schema and available tool nodes.

---

## 🐍 Local Python Skills

Place skill folders under `skills/`, each containing:

```
skills/
  my_skill/
    metadata.json   # name, description, parameters schema, script filename, sensitive flag
    my_skill.py     # executable script
```

Skills are scanned at startup and registered in the tool catalog. Example included: `skills/ping_host/`.

---

## ⚡ Smart Context Cache (SCC) — Experimental

> **Status: Layer 2 determinism validated (automated tests).** Live provider cache-hit metrics still require optional manual verification with Anthropic/OpenRouter. See [docs/corporate-roadmap/SCC-TEST-RESULTS.md](docs/corporate-roadmap/SCC-TEST-RESULTS.md).

Smart Context Cache organizes each LLM request into three volatility layers to maximize **context caching** across providers (DeepSeek, Anthropic, Gemini, OpenAI, etc.):

| Layer | Content | Volatility |
|-------|---------|------------|
| **Layer 1** | System prompt, tool contracts, RuleGo spec | Immutable (0%) |
| **Layer 2** | Session goals, local workspace files, uploaded session files | Semi-static (low) |
| **Layer 3** | Chat history and current user message | Dynamic (100%) |

### What you can configure today (Layer 2)

From the chat panel (⚡ toggle):

- **Session goals** — focus instructions for the current conversation (e.g. *"Today we only refine unit tests"*).
- **Local workspace** — path to a project directory, with a folder browser. Commands are sandboxed to stay within the workspace boundary.
- **Session files** — upload PDF, HTML, TXT, MD, JSON, CSV, or XLSX files attached to the session context.

API endpoints: `POST /api/sessions/:id/goals`, `POST /api/sessions/:id/workspace`, `POST/GET/DELETE /api/sessions/:id/files`, `GET /api/workspace/browse`.

See `docs/smart-context-cache-specification.md` for the full technical design (provider-specific cache adapters, deterministic hashing, tail compacting, etc.).

---

## 🛠️ Tech Stack

- **Frontend**: Angular 21 (Signals, Standalone Components, vanilla CSS).
- **Backend**: Go 1.25+ (Fiber, GORM).
- **Database**: PostgreSQL with **pgvector** (`pgvector/pgvector:pg15`).
- **AI**: Agentic orchestration via OpenRouter / Groq / Gemini and other OpenAI-compatible providers.
- **Workflows**: RuleGo engine with Mermaid visualization.
- **RAG**: LangChainGo document parsing + pgvector similarity search.
- **Infrastructure**: Docker & Docker Compose (ready for **CubePath**).

---

## 🚀 Installation & Deployment

### Prerequisites
- Docker and Docker Compose installed.
- A modern browser.

### Deployment Steps

1. **Configuration**: Copy the example file and set `DB_ENCRYPTION_KEY` (exactly 32 characters for AES-256), `JWT_SECRET` (session signing, at least 16 characters), plus `ADMIN_USERNAME` and `ADMIN_PASSWORD`. See [docs/corporate-roadmap/SECRETS.md](docs/corporate-roadmap/SECRETS.md) for migration from older single-key setups.
   ```bash
   cp .env.example .env
   ```
2. **Start the system**: Docker Compose builds and runs the app (API + Angular static files) and the database.
   ```bash
   docker-compose up -d --build
   ```
3. **Access**:
   - **Production (Docker)**: `http://localhost:3000` (API and UI on the same port)
   - **Local development**: Angular dev server at `http://localhost:4200` (proxied to the Go API on `:3000`)

---

## 📖 How It Works

1. **Configure your brain**: In **LLM Providers**, add one or more providers for redundancy. Mark one as **default** and optionally one as **Embeddings Provider** for RAG. Use the model dropdown with refresh to pick models dynamically. Keys are encrypted after a successful connection test.
2. **Connect your hands**: Configure the **HandsAI** bridge URL and token. Optionally register additional **MCP stdio** and **MCP stream** servers to expand the tool catalog.
3. **Set rules**: Define behavioral rules like *"Always be concise"* or *"Validate the Odoo ID before creating anything"*.
4. **Agents & tools**: In **Agents**, define each personality's model/provider and tool subset. Switch agents per session in chat. Reset LLM overrides if a fallback or manual override was applied.
5. **Upload knowledge (RAG)**: Upload documents so the agent can answer from your data. Relevant chunks are injected automatically on each message.
6. **Smart Context Cache (optional)**: Set session goals, point to a local workspace, and attach session files for long-running, context-heavy work. Layer 2 hash stability is covered by automated tests; see [SCC-TEST-RESULTS.md](docs/corporate-roadmap/SCC-TEST-RESULTS.md).
7. **Automate**: Ask for complex tasks — *"Create a Trello card on the Hackathon board and register it in Odoo"* — or create **scheduled tasks** and **RuleGo workflows** from the Dashboard / Workflows tabs.
8. **Approve & permission**: Sensitive actions pause for confirmation. Check **Always allow** to persist permission, or manage all permissions from the **Permissions** tab. Pending actions across sessions appear in **Approvals**.

---

## ⚠️ Note on Model Selection

> AIgent's performance depends directly on the reasoning capabilities of the configured model. Models **under 100B parameters** work well for simple tasks and direct queries, but may struggle to chain complex multi-tool execution flows. For full orchestration, models of **100B parameters or more** are recommended.
