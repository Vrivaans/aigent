import { Injectable } from '@angular/core';

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
  created_at: string;
  updated_at: string;
}

export interface Rule {
  id: number;
  category: string;
  content: string;
  importance: number;
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
  ID: number;
  name: string;
  base_url: string;
  api_key: string;
  default_model: string;
  is_active: boolean;
  is_default: boolean;
}

@Injectable({ providedIn: 'root' })
export class ApiService {
  private baseUrl = 'http://localhost:3000/api';

  private get headers() {
    const token = localStorage.getItem('aigent_token');
    return {
      'Content-Type': 'application/json',
      ...(token ? { 'Authorization': `Bearer ${token}` } : {})
    };
  }

  async login(username: string, password: string): Promise<{ token: string }> {
    const res = await fetch(`${this.baseUrl}/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Login failed');
    return data;
  }

  async getSessions(): Promise<Session[]> {
    return fetch(`${this.baseUrl}/sessions`, { headers: this.headers }).then(res => res.json());
  }

  async createSession(): Promise<Session> {
    return fetch(`${this.baseUrl}/sessions`, { 
      method: 'POST',
      headers: this.headers 
    }).then(res => res.json());
  }

  async getChatHistory(sessionId: number): Promise<ChatMessage[]> {
    return fetch(`${this.baseUrl}/sessions/${sessionId}/chat`, {
      headers: this.headers
    }).then(res => res.json());
  }

  async sendChatMessage(sessionId: number, message: string): Promise<{
    response: string, 
    tool_calls: any[], 
    requires_confirmation?: boolean, 
    pending_action_id?: number, 
    waiting_tool?: any
  }> {
    return fetch(`${this.baseUrl}/sessions/${sessionId}/chat`, {
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify({ message })
    }).then(res => res.json());
  }

  async confirmAction(sessionId: number, pendingId: number, approved: boolean): Promise<any> {
    const res = await fetch(`${this.baseUrl}/sessions/${sessionId}/confirm/${pendingId}`, {
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify({ approved })
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || 'Server error');
    }
    return data;
  }

  async getRules(): Promise<Rule[]> {
    return fetch(`${this.baseUrl}/rules`, { headers: this.headers }).then(res => res.json());
  }

  async createRule(rule: Partial<Rule>): Promise<Rule> {
    return fetch(`${this.baseUrl}/rules`, {
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify(rule)
    }).then(res => res.json());
  }

  async deleteRule(id: number) {
    return fetch(`${this.baseUrl}/rules/${id}`, { 
      method: 'DELETE',
      headers: this.headers 
    }).then(res => res.json());
  }

  async getTasks(): Promise<Task[]> {
    return fetch(`${this.baseUrl}/tasks`, { headers: this.headers }).then(res => res.json());
  }

  async getActiveTools(): Promise<any[]> {
    const res = await fetch(`${this.baseUrl}/active-tools`, { headers: this.headers });
    return res.json();
  }

  // LLM Providers
  async getProviders(): Promise<LLMProvider[]> {
    const res = await fetch(`${this.baseUrl}/providers`, { headers: this.headers });
    return res.json();
  }

  async createProvider(provider: Partial<LLMProvider>): Promise<LLMProvider> {
    const res = await fetch(`${this.baseUrl}/providers`, {
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify(provider)
    });
    return res.json();
  }

  async updateProvider(id: number, provider: Partial<LLMProvider>): Promise<LLMProvider> {
    const res = await fetch(`${this.baseUrl}/providers/${id}`, {
      method: 'PATCH',
      headers: this.headers,
      body: JSON.stringify(provider)
    });
    return res.json();
  }

  async setDefaultProvider(id: number): Promise<void> {
    await fetch(`${this.baseUrl}/providers/${id}/set-default`, {
      method: 'PATCH',
      headers: this.headers
    });
  }

  async deleteProvider(id: number): Promise<void> {
    await fetch(`${this.baseUrl}/providers/${id}`, { 
      method: 'DELETE',
      headers: this.headers 
    });
  }

  async testProvider(config: any): Promise<{ ok: boolean; message: string }> {
    const res = await fetch(`${this.baseUrl}/providers/test`, { 
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify(config)
    });
    return res.json();
  }

  async deleteTask(id: number) {
    return fetch(`${this.baseUrl}/tasks/${id}`, { 
      method: 'DELETE',
      headers: this.headers 
    }).then(res => res.json());
  }
}
