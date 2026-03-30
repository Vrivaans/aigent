import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, LLMProvider, McpStdioServer } from '../api.service';

@Component({
  selector: 'app-providers',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './providers.html',
  styleUrl: './providers.css'
})
export class Providers implements OnInit {
  private api = inject(ApiService);
  
  providers = signal<LLMProvider[]>([]);
  showAddForm = signal(false);
  isEditing = signal(false);
  editingId = signal<number | null>(null);
  testResult = signal<{ ok: boolean; message: string } | null>(null);
  isTesting = signal(false);
  
  newProvider: Partial<LLMProvider> = {
    name: '',
    base_url: 'https://api.groq.com/openai/v1',
    api_key: '',
    default_model: 'llama-3.3-70b-versatile'
  };

  handsaiConfig = signal({ url: '', token: '' });

  mcpStdioServers = signal<McpStdioServer[]>([]);
  showMcpModal = signal(false);
  mcpEditingId = signal<number | null>(null);
  mcpForm: {
    alias: string;
    command: string;
    argsText: string;
    envText: string;
    enabled: boolean;
  } = {
    alias: '',
    command: 'npx',
    argsText: '',
    envText: '',
    enabled: true
  };
  mcpTestMsg = signal<string | null>(null);
  mcpTesting = signal(false);
  
  async ngOnInit() {
    await this.loadProviders();
    await this.loadHandsAIConfig();
    await this.loadMcpStdio();
  }

  async loadProviders() {
    const p = await this.api.getProviders();
    this.providers.set(p);
  }

  async loadHandsAIConfig() {
    const config = await this.api.getHandsAIConfig();
    this.handsaiConfig.set(config);
  }

  async loadMcpStdio() {
    try {
      const list = await this.api.listMcpStdioServers();
      this.mcpStdioServers.set(list);
    } catch {
      this.mcpStdioServers.set([]);
    }
  }

  envMapToText(env: Record<string, string> | undefined): string {
    if (!env || !Object.keys(env).length) return '';
    return Object.entries(env)
      .map(([k, v]) => `${k}=${v}`)
      .join('\n');
  }

  parseArgsText(text: string): string[] {
    return text
      .split('\n')
      .map((l) => l.trim())
      .filter(Boolean);
  }

  parseEnvText(text: string): Record<string, string> {
    const out: Record<string, string> = {};
    for (const line of text.split('\n')) {
      const t = line.trim();
      if (!t) continue;
      const i = t.indexOf('=');
      if (i <= 0) continue;
      const k = t.slice(0, i).trim();
      const v = t.slice(i + 1).trim();
      if (k) out[k] = v;
    }
    return out;
  }

  openNewMcp() {
    this.mcpEditingId.set(null);
    this.mcpTestMsg.set(null);
    this.mcpForm = {
      alias: '',
      command: 'npx',
      argsText: '-y\n@modelcontextprotocol/server-filesystem\n/ruta/permitida',
      envText: '',
      enabled: true
    };
    this.showMcpModal.set(true);
  }

  editMcp(s: McpStdioServer) {
    this.mcpEditingId.set(s.id);
    this.mcpTestMsg.set(null);
    this.mcpForm = {
      alias: s.alias,
      command: s.command,
      argsText: (s.args || []).join('\n'),
      envText: this.envMapToText(s.env),
      enabled: s.enabled
    };
    this.showMcpModal.set(true);
  }

  closeMcpModal() {
    this.showMcpModal.set(false);
    this.mcpEditingId.set(null);
    this.mcpTestMsg.set(null);
  }

  async saveMcp() {
    const alias = this.mcpForm.alias.trim();
    const command = this.mcpForm.command.trim();
    if (!alias || !command) {
      alert('Alias y comando son obligatorios.');
      return;
    }
    const args = this.parseArgsText(this.mcpForm.argsText);
    const env = this.parseEnvText(this.mcpForm.envText);
    try {
      const id = this.mcpEditingId();
      if (id != null) {
        await this.api.updateMcpStdioServer(id, {
          alias,
          command,
          args,
          env,
          enabled: this.mcpForm.enabled
        });
      } else {
        await this.api.createMcpStdioServer({
          alias,
          command,
          args,
          env,
          enabled: this.mcpForm.enabled
        });
      }
      await this.loadMcpStdio();
      this.closeMcpModal();
    } catch (e: any) {
      alert(e?.message || 'Error al guardar');
    }
  }

  async deleteMcp(s: McpStdioServer, ev: Event) {
    ev.stopPropagation();
    if (!confirm(`¿Eliminar servidor MCP «${s.alias}»?`)) return;
    try {
      await this.api.deleteMcpStdioServer(s.id);
      await this.loadMcpStdio();
    } catch (e: any) {
      alert(e?.message || 'Error al eliminar');
    }
  }

  async testMcpDryRun() {
    const command = this.mcpForm.command.trim();
    if (!command) {
      alert('Indica un comando para probar.');
      return;
    }
    this.mcpTesting.set(true);
    this.mcpTestMsg.set(null);
    try {
      const args = this.parseArgsText(this.mcpForm.argsText);
      const env = this.parseEnvText(this.mcpForm.envText);
      const r = await this.api.testMcpStdioDryRun({ command, args, env });
      this.mcpTestMsg.set(`OK: ${r.tools?.length ?? 0} tools — ${(r.tools || []).join(', ')}`);
    } catch (e: any) {
      this.mcpTestMsg.set(e?.message || 'Error');
    } finally {
      this.mcpTesting.set(false);
    }
  }

  async testMcpSaved() {
    const id = this.mcpEditingId();
    if (id == null) return;
    this.mcpTesting.set(true);
    this.mcpTestMsg.set(null);
    try {
      const r = await this.api.testMcpStdioSaved(id);
      this.mcpTestMsg.set(`OK [${r.alias}]: ${r.tools?.length ?? 0} tools — ${(r.tools || []).join(', ')}`);
    } catch (e: any) {
      this.mcpTestMsg.set(e?.message || 'Error');
    } finally {
      this.mcpTesting.set(false);
    }
  }

  async saveHandsAIConfig() {
    await this.api.updateHandsAIConfig(this.handsaiConfig());
    alert('Configuración de HandsAI guardada correctamente.');
    await this.loadHandsAIConfig();
  }

  async deleteHandsAIConfig() {
    if (!confirm('¿Eliminar la configuración de HandsAI? Las herramientas dejarán de estar disponibles hasta que vuelvas a configurarlo.')) return;
    await this.api.deleteHandsAIConfig();
    this.handsaiConfig.set({ url: '', token: '' });
  }

  async onSaveProvider() {
    if (!this.newProvider.name) return;
    
    if (this.isEditing() && this.editingId()) {
      await this.api.updateProvider(this.editingId()!, this.newProvider);
    } else {
      await this.api.createProvider(this.newProvider);
    }

    await this.loadProviders();
    this.showAddForm.set(false);
    this.resetForm();
  }

  editProvider(p: LLMProvider) {
    this.isEditing.set(true);
    this.editingId.set(p.id);
    this.newProvider = {
      name: p.name,
      base_url: p.base_url,
      api_key: '********', // Placeholder to indicate we have a key
      default_model: p.default_model
    };
    this.showAddForm.set(true);
  }

  resetForm() {
    this.isEditing.set(false);
    this.editingId.set(null);
    this.newProvider = {
      name: '',
      base_url: 'https://api.groq.com/openai/v1',
      api_key: '',
      default_model: 'llama-3.3-70b-versatile'
    };
  }

  async setAsDefault(id: number, event: Event) {
    event.stopPropagation();
    await this.api.setDefaultProvider(id);
    await this.loadProviders();
  }

  async deleteProvider(id: number, event: Event) {
    event.stopPropagation();
    if (!confirm('¿Eliminar este proveedor?')) return;
    await this.api.deleteProvider(id);
    await this.loadProviders();
  }

  async testProvider() {
    const config = {
      ...this.newProvider,
      id: this.editingId() || 0
    };
    
    this.isTesting.set(true);
    this.testResult.set(null);
    try {
      const res = await this.api.testProvider(config);
      this.testResult.set(res);
    } catch (e: any) {
      this.testResult.set({ ok: false, message: e.message || 'Error desconocido' });
    } finally {
      this.isTesting.set(false);
    }
  }
}
