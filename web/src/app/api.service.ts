import { inject, Injectable } from '@angular/core';

import { environment } from '../environments/environment';
import { AuthService } from './auth/auth.service';

export interface ChatMessage {
  id: number;
  role: string;
  content: string;
  created_at: string;
  tool_calls?: any[];
}

export interface Session {
  id: number;
  title: string;
  agent_id?: number;
  agent?: Agent;
  created_at: string;
  updated_at: string;
}

export interface Rule {
  id: number;
  agents?: Agent[];
  category: string;
  content: string;
  importance: number;
}

export interface Agent {
  id: number;
  name: string;
  description: string;
  llm_provider_id: number;
  llm_provider?: LLMProvider;
  tools?: AgentTool[];
  is_default: boolean;
  created_at?: string;
}

export interface AgentTool {
  id: number;
  agent_id: number;
  tool_name: string;
}

export interface Task {
  id: number;
  name: string;
  cron_expression: string;
  tool_name: string;
  payload: any;
  next_run_at: string;
}

export interface LLMProvider {
  id: number;
  name: string;
  base_url: string;
  api_key: string;
  default_model: string;
  is_active: boolean;
  is_default: boolean;
}

export interface McpStdioServer {
  id: number;
  alias: string;
  command: string;
  args: string[];
  env: Record<string, string>;
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface McpStreamServer {
  id: number;
  alias: string;
  base_url: string;
  headers: Record<string, string>;
  disable_standalone_sse: boolean;
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
}

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly baseUrl = environment.apiBaseUrl;
  private readonly auth = inject(AuthService);

  private get headers() {
    const token = localStorage.getItem('aigent_token');
    return {
      'Content-Type': 'application/json',
      ...(token ? { 'Authorization': `Bearer ${token}` } : {})
    };
  }

  /**
   * Peticiones autenticadas. Si `isLogin` es true, un 401 no cierra sesión (credenciales incorrectas).
   */
  private async fetchApi(path: string, init: RequestInit = {}, isLogin = false): Promise<Response> {
    const headers = isLogin
      ? { 'Content-Type': 'application/json', ...init.headers as Record<string, string> }
      : { ...this.headers, ...init.headers };
    const res = await fetch(`${this.baseUrl}${path}`, { ...init, headers });
    if (!isLogin && res.status === 401) {
      this.auth.logout();
    }
    return res;
  }

  private async request(endpoint: string, options: RequestInit = {}): Promise<any> {
    const res = await this.fetchApi(endpoint, options, false);
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async login(username: string, password: string): Promise<{ token: string }> {
    const res = await this.fetchApi(
      '/login',
      {
        method: 'POST',
        body: JSON.stringify({ username, password })
      },
      true
    );
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Login failed');
    return data;
  }

  async getSessions(): Promise<Session[]> {
    const res = await this.fetchApi('/sessions');
    return res.json();
  }

  async createSession(): Promise<Session> {
    const res = await this.fetchApi('/sessions', { method: 'POST' });
    return res.json();
  }

  async deleteSession(sessionId: number): Promise<void> {
    await this.fetchApi(`/sessions/${sessionId}`, { method: 'DELETE' });
  }

  async updateSessionAgent(sessionId: number, agentId: number): Promise<any> {
    const res = await this.fetchApi(`/sessions/${sessionId}/agent`, {
      method: 'PATCH',
      body: JSON.stringify({ agent_id: agentId })
    });
    if (!res.ok) throw new Error(await res.text());
    return res.json();
  }

  async getChatHistory(sessionId: number): Promise<ChatMessage[]> {
    const res = await this.fetchApi(`/sessions/${sessionId}/chat`);
    return res.json();
  }

  async sendChatMessage(sessionId: number, message: string): Promise<{
    response: string,
    tool_calls: any[],
    requires_confirmation?: boolean,
    pending_action_id?: number,
    waiting_tool?: any
  }> {
    const res = await this.fetchApi(`/sessions/${sessionId}/chat`, {
      method: 'POST',
      body: JSON.stringify({ message })
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(typeof data?.error === 'string' ? data.error : 'Error del servidor');
    }
    return data;
  }

  async confirmAction(sessionId: number, pendingId: number, approved: boolean): Promise<any> {
    const res = await this.fetchApi(`/sessions/${sessionId}/confirm/${pendingId}`, {
      method: 'POST',
      body: JSON.stringify({ approved })
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || 'Server error');
    }
    return data;
  }

  async getRules(): Promise<Rule[]> {
    const res = await this.fetchApi('/rules');
    return res.json();
  }

  async createRule(rule: Partial<Rule>): Promise<Rule> {
    const res = await this.fetchApi('/rules', {
      method: 'POST',
      body: JSON.stringify(rule)
    });
    return res.json();
  }

  async deleteRule(id: number) {
    return this.request(`/rules/${id}`, { method: 'DELETE' });
  }

  // --- Agents ---
  async getAgents(): Promise<Agent[]> {
    return this.request('/admin/agents');
  }

  async getProviders(): Promise<LLMProvider[]> {
    return this.request('/providers');
  }

  async getActiveTools(): Promise<any[]> {
    return this.request('/active-tools');
  }

  async createAgent(data: any): Promise<Agent> {
    return this.request('/admin/agents', {
      method: 'POST',
      body: JSON.stringify(data)
    });
  }

  async updateAgent(id: number, data: any): Promise<Agent> {
    return this.request(`/admin/agents/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data)
    });
  }

  async deleteAgent(id: number) {
    return this.request(`/admin/agents/${id}`, {
      method: 'DELETE'
    });
  }

  async getTasks(): Promise<Task[]> {
    return this.request('/tasks');
  }

  async createProvider(provider: Partial<LLMProvider>): Promise<LLMProvider> {
    const res = await this.fetchApi('/providers', {
      method: 'POST',
      body: JSON.stringify(provider)
    });
    return res.json();
  }

  async updateProvider(id: number, provider: Partial<LLMProvider>): Promise<LLMProvider> {
    const res = await this.fetchApi(`/providers/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(provider)
    });
    return res.json();
  }

  async setDefaultProvider(id: number): Promise<void> {
    await this.fetchApi(`/providers/${id}/set-default`, { method: 'PATCH' });
  }

  async deleteProvider(id: number): Promise<void> {
    await this.fetchApi(`/providers/${id}`, { method: 'DELETE' });
  }

  async testProvider(config: any): Promise<{ ok: boolean; message: string }> {
    const res = await this.fetchApi('/providers/test', {
      method: 'POST',
      body: JSON.stringify(config)
    });
    return res.json();
  }

  async deleteTask(id: number) {
    const res = await this.fetchApi(`/tasks/${id}`, { method: 'DELETE' });
    return res.json();
  }

  // HandsAI Config
  async getHandsAIConfig(): Promise<{ url: string; token: string }> {
    const res = await this.fetchApi('/config/handsai');
    return res.json();
  }

  async updateHandsAIConfig(config: { url: string; token: string }): Promise<any> {
    const res = await this.fetchApi('/config/handsai', {
      method: 'PATCH',
      body: JSON.stringify(config)
    });
    return res.json();
  }

  async deleteHandsAIConfig(): Promise<any> {
    const res = await this.fetchApi('/config/handsai', { method: 'DELETE' });
    return res.json();
  }

  async listMcpStdioServers(): Promise<McpStdioServer[]> {
    return this.request('/config/mcp-stdio');
  }

  async createMcpStdioServer(body: {
    alias: string;
    command: string;
    args: string[];
    env: Record<string, string>;
    enabled?: boolean;
  }): Promise<{ status: string; id: number }> {
    return this.request('/config/mcp-stdio', { method: 'POST', body: JSON.stringify(body) });
  }

  async updateMcpStdioServer(
    id: number,
    body: Partial<{
      alias: string;
      command: string;
      args: string[];
      env: Record<string, string>;
      enabled: boolean;
    }>
  ): Promise<{ status: string }> {
    return this.request(`/config/mcp-stdio/${id}`, { method: 'PATCH', body: JSON.stringify(body) });
  }

  async deleteMcpStdioServer(id: number): Promise<{ status: string }> {
    return this.request(`/config/mcp-stdio/${id}`, { method: 'DELETE' });
  }

  async testMcpStdioDryRun(body: {
    command: string;
    args: string[];
    env: Record<string, string>;
  }): Promise<{ ok: boolean; tools: string[]; error?: string }> {
    const res = await fetch(`${this.baseUrl}/config/mcp-stdio/test`, {
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify(body)
    });
    let data: any = {};
    try {
      data = await res.json();
    } catch {
      /* ignore */
    }
    if (!res.ok) throw new Error(data.error || 'Test failed');
    return data;
  }

  async testMcpStdioSaved(id: number): Promise<{ ok: boolean; tools: string[]; alias?: string }> {
    const res = await fetch(`${this.baseUrl}/config/mcp-stdio/${id}/test`, {
      method: 'POST',
      headers: this.headers
    });
    let data: any = {};
    try {
      data = await res.json();
    } catch {
      /* ignore */
    }
    if (!res.ok) throw new Error(data.error || 'Test failed');
    return data;
  }

  async listMcpStreamServers(): Promise<McpStreamServer[]> {
    return this.request('/config/mcp-stream');
  }

  async createMcpStreamServer(body: {
    alias: string;
    base_url: string;
    headers?: Record<string, string>;
    disable_standalone_sse?: boolean;
    enabled?: boolean;
  }): Promise<{ status: string; id: number }> {
    return this.request('/config/mcp-stream', { method: 'POST', body: JSON.stringify(body) });
  }

  async updateMcpStreamServer(
    id: number,
    body: Partial<{
      alias: string;
      base_url: string;
      headers: Record<string, string>;
      disable_standalone_sse: boolean;
      enabled: boolean;
    }>
  ): Promise<{ status: string }> {
    return this.request(`/config/mcp-stream/${id}`, { method: 'PATCH', body: JSON.stringify(body) });
  }

  async deleteMcpStreamServer(id: number): Promise<{ status: string }> {
    return this.request(`/config/mcp-stream/${id}`, { method: 'DELETE' });
  }

  async testMcpStreamDryRun(body: {
    base_url: string;
    headers?: Record<string, string>;
    disable_standalone_sse?: boolean;
  }): Promise<{ ok: boolean; tools: string[]; error?: string }> {
    const res = await fetch(`${this.baseUrl}/config/mcp-stream/test`, {
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify(body)
    });
    let data: any = {};
    try {
      data = await res.json();
    } catch {
      /* ignore */
    }
    if (!res.ok) throw new Error(data.error || 'Test failed');
    return data;
  }

  async testMcpStreamSaved(id: number): Promise<{ ok: boolean; tools: string[]; alias?: string }> {
    const res = await fetch(`${this.baseUrl}/config/mcp-stream/${id}/test`, {
      method: 'POST',
      headers: this.headers
    });
    let data: any = {};
    try {
      data = await res.json();
    } catch {
      /* ignore */
    }
    if (!res.ok) throw new Error(data.error || 'Test failed');
    return data;
  }
}
