import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, LLMProvider, McpStdioServer, McpStreamServer, ModelInfo } from '../api.service';

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
    base_url: 'https://opencode.ai/zen/v1',
    api_key: '',
    default_model: 'big-pickle',
    provider_type: 'zen'
  };

  providerModels = signal<ModelInfo[]>([]);
  modelsLoading = signal(false);
  modelsError = signal<string | null>(null);

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

  mcpStreamServers = signal<McpStreamServer[]>([]);
  showMcpStreamModal = signal(false);
  mcpStreamEditingId = signal<number | null>(null);
  mcpStreamForm: {
    alias: string;
    baseUrl: string;
    headersText: string;
    disableStandaloneSSE: boolean;
    enabled: boolean;
  } = {
    alias: '',
    baseUrl: '',
    headersText: '',
    disableStandaloneSSE: false,
    enabled: true
  };
  mcpStreamTestMsg = signal<string | null>(null);
  mcpStreamTesting = signal(false);

  async ngOnInit() {
    await this.loadProviders();
    await this.loadHandsAIConfig();
    await this.loadMcpStdio();
    await this.loadMcpStream();
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

  async loadMcpStream() {
    try {
      const list = await this.api.listMcpStreamServers();
      this.mcpStreamServers.set(list);
    } catch {
      this.mcpStreamServers.set([]);
    }
  }

  headersMapToText(h: Record<string, string> | undefined): string {
    if (!h || !Object.keys(h).length) return '';
    return Object.entries(h)
      .map(([k, v]) => `${k}: ${v}`)
      .join('\n');
  }

  parseHeadersText(text: string): Record<string, string> {
    const out: Record<string, string> = {};
    for (const line of text.split('\n')) {
      const t = line.trim();
      if (!t) continue;
      const i = t.indexOf(':');
      if (i <= 0) continue;
      const k = t.slice(0, i).trim();
      const v = t.slice(i + 1).trim();
      if (k) out[k] = v;
    }
    return out;
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

  openNewMcpStream() {
    this.mcpStreamEditingId.set(null);
    this.mcpStreamTestMsg.set(null);
    this.mcpStreamForm = {
      alias: '',
      baseUrl: 'https://',
      headersText: '',
      disableStandaloneSSE: false,
      enabled: true
    };
    this.showMcpStreamModal.set(true);
  }

  editMcpStream(s: McpStreamServer) {
    this.mcpStreamEditingId.set(s.id);
    this.mcpStreamTestMsg.set(null);
    this.mcpStreamForm = {
      alias: s.alias,
      baseUrl: s.base_url,
      headersText: this.headersMapToText(s.headers),
      disableStandaloneSSE: s.disable_standalone_sse,
      enabled: s.enabled
    };
    this.showMcpStreamModal.set(true);
  }

  closeMcpStreamModal() {
    this.showMcpStreamModal.set(false);
    this.mcpStreamEditingId.set(null);
    this.mcpStreamTestMsg.set(null);
  }

  async saveMcpStream() {
    const alias = this.mcpStreamForm.alias.trim();
    const baseUrl = this.mcpStreamForm.baseUrl.trim();
    if (!alias || !baseUrl) {
      alert('Alias y URL base son obligatorios.');
      return;
    }
    const headers = this.parseHeadersText(this.mcpStreamForm.headersText);
    try {
      const id = this.mcpStreamEditingId();
      if (id != null) {
        await this.api.updateMcpStreamServer(id, {
          alias,
          base_url: baseUrl,
          headers,
          disable_standalone_sse: this.mcpStreamForm.disableStandaloneSSE,
          enabled: this.mcpStreamForm.enabled
        });
      } else {
        await this.api.createMcpStreamServer({
          alias,
          base_url: baseUrl,
          headers,
          disable_standalone_sse: this.mcpStreamForm.disableStandaloneSSE,
          enabled: this.mcpStreamForm.enabled
        });
      }
      await this.loadMcpStream();
      this.closeMcpStreamModal();
    } catch (e: any) {
      alert(e?.message || 'Error al guardar');
    }
  }

  async deleteMcpStream(s: McpStreamServer, ev: Event) {
    ev.stopPropagation();
    if (!confirm(`¿Eliminar servidor MCP remoto «${s.alias}»?`)) return;
    try {
      await this.api.deleteMcpStreamServer(s.id);
      await this.loadMcpStream();
    } catch (e: any) {
      alert(e?.message || 'Error al eliminar');
    }
  }

  async testMcpStreamDryRun() {
    const baseUrl = this.mcpStreamForm.baseUrl.trim();
    if (!baseUrl) {
      alert('Indica la URL base del servidor MCP.');
      return;
    }
    this.mcpStreamTesting.set(true);
    this.mcpStreamTestMsg.set(null);
    try {
      const headers = this.parseHeadersText(this.mcpStreamForm.headersText);
      const r = await this.api.testMcpStreamDryRun({
        base_url: baseUrl,
        headers,
        disable_standalone_sse: this.mcpStreamForm.disableStandaloneSSE
      });
      this.mcpStreamTestMsg.set(`OK: ${r.tools?.length ?? 0} tools — ${(r.tools || []).join(', ')}`);
    } catch (e: any) {
      this.mcpStreamTestMsg.set(e?.message || 'Error');
    } finally {
      this.mcpStreamTesting.set(false);
    }
  }

  async testMcpStreamSaved() {
    const id = this.mcpStreamEditingId();
    if (id == null) return;
    this.mcpStreamTesting.set(true);
    this.mcpStreamTestMsg.set(null);
    try {
      const r = await this.api.testMcpStreamSaved(id);
      this.mcpStreamTestMsg.set(`OK [${r.alias}]: ${r.tools?.length ?? 0} tools — ${(r.tools || []).join(', ')}`);
    } catch (e: any) {
      this.mcpStreamTestMsg.set(e?.message || 'Error');
    } finally {
      this.mcpStreamTesting.set(false);
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

  async onSaveAndRefreshModels() {
    if (!this.newProvider.name) return;
    
    let savedId: number | null = null;
    if (this.isEditing() && this.editingId()) {
      savedId = this.editingId()!;
      await this.api.updateProvider(savedId, this.newProvider);
    } else {
      const created = await this.api.createProvider(this.newProvider);
      savedId = created.id;
    }

    await this.loadProviders();
    this.showAddForm.set(false);

    if (savedId) {
      this.editingId.set(savedId);
      await this.refreshModelsForProvider(savedId);
      const p = this.providers().find(pr => pr.id === savedId);
      if (p) {
        this.isEditing.set(true);
        this.editingId.set(p.id);
        this.newProvider = {
          name: p.name,
          base_url: p.base_url,
          api_key: '********',
          default_model: p.default_model,
          provider_type: p.provider_type
        };
        this.showAddForm.set(true);
      }
    }
  }

  editProvider(p: LLMProvider) {
    this.isEditing.set(true);
    this.editingId.set(p.id);
    this.newProvider = {
      name: p.name,
      base_url: p.base_url,
      api_key: '********',
      default_model: p.default_model,
      provider_type: p.provider_type
    };
    this.loadProviderModels();
    this.showAddForm.set(true);
  }

  resetForm() {
    this.isEditing.set(false);
    this.editingId.set(null);
    this.newProvider = {
      name: '',
      base_url: 'https://opencode.ai/zen/v1',
      api_key: '',
      default_model: 'big-pickle',
      provider_type: 'zen'
    };
    this.providerModels.set([]);
    this.modelsError.set(null);
  }

  async onProviderTypeChange() {
    const type = this.newProvider.provider_type;
    const presets = await this.api.getProviderPresets();
    const preset = presets.find(p => p.type === type);
    if (preset && preset.base_url) {
      this.newProvider.base_url = preset.base_url;
    } else if (type === 'custom') {
      this.newProvider.base_url = '';
    }
    this.providerModels.set([]);
    this.modelsError.set(null);
  }

  async loadProviderModels() {
    const id = this.editingId();
    if (id != null && id > 0) {
      this.modelsLoading.set(true);
      this.modelsError.set(null);
      try {
        const models = await this.api.getProviderModels(id);
        this.providerModels.set(models);
        if (models.length === 0) {
          this.modelsError.set('No se encontraron modelos para este proveedor. Verificá la conexión y probá refrescar.');
        }
      } catch (e: any) {
        this.providerModels.set([]);
        this.modelsError.set('No se pudo obtener la lista de modelos. Podés escribir el nombre manualmente.');
      } finally {
        this.modelsLoading.set(false);
      }
    } else {
      this.tryFetchModelsFromEndpoint();
    }
  }

  async tryFetchModelsFromEndpoint() {
    const baseUrl = this.newProvider.base_url?.trim();
    const apiKey = this.newProvider.api_key;
    if (!baseUrl) {
      this.providerModels.set([]);
      this.modelsError.set('Ingresá la Base URL para cargar los modelos disponibles.');
      return;
    }
    if (apiKey === '********' || !apiKey) {
      this.providerModels.set([]);
      this.modelsError.set('Ingresá la API Key para cargar los modelos disponibles.');
      return;
    }
    this.modelsLoading.set(true);
    this.modelsError.set(null);
    try {
      await this.api.testProvider({
        ...this.newProvider,
        id: this.editingId() || 0
      });
      const id = this.editingId();
      if (id != null && id > 0) {
        const result = await this.api.refreshProviderModels(id);
        this.providerModels.set(result.models || []);
        if (!result.models?.length) {
          this.modelsError.set('El proveedor no expone un endpoint /models. Escribí el nombre del modelo manualmente.');
        }
      } else {
        this.providerModels.set([]);
        this.modelsError.set('Guardá el proveedor primero, luego podés refrescar la lista de modelos.');
      }
    } catch {
      this.providerModels.set([]);
      this.modelsError.set('No se puede visualizar la lista de modelos. Escribí el nombre del modelo manualmente.');
    } finally {
      this.modelsLoading.set(false);
    }
  }

  async refreshModelsForProvider(providerId: number) {
    this.modelsLoading.set(true);
    this.modelsError.set(null);
    try {
      const result = await this.api.refreshProviderModels(providerId);
      this.providerModels.set(result.models || []);
      if (!result.models?.length) {
        this.modelsError.set('El proveedor no expone un endpoint /models. Escribí el nombre del modelo manualmente.');
      }
    } catch {
      this.providerModels.set([]);
      this.modelsError.set('No se puede visualizar la lista de modelos en /models. Escribí el nombre del modelo manualmente.');
    } finally {
      this.modelsLoading.set(false);
    }
  }

  async setAsDefault(id: number, event: Event) {
    event.stopPropagation();
    await this.api.setDefaultProvider(id);
    await this.loadProviders();
  }

  async refreshModelsForCard(p: LLMProvider, event: Event) {
    event.stopPropagation();
    try {
      const result = await this.api.refreshProviderModels(p.id);
      alert(result.ok ? `✅ ${result.models?.length ?? 0} modelos cargados` : `⚠️ ${result.message}`);
    } catch (e: any) {
      alert('Error al refrescar modelos: ' + (e?.message || 'desconocido'));
    }
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
