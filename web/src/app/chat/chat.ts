import { Component, signal, inject, OnInit, ViewChild, ElementRef, AfterViewChecked, Input, Output, EventEmitter, OnChanges, SimpleChanges, computed } from '@angular/core';
import { ApiService, ChatMessage, Session, Agent, ProviderSwitchInfo, ModelGroup, ModelInfo } from '../api.service';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { DiagramRenderer } from './diagram-renderer';

export interface ChatMessageUI extends ChatMessage {
  requires_confirmation?: boolean;
  pending_action_id?: number;
  waiting_tool?: any;
  confirmed?: boolean;
  rejected?: boolean;
  provider_switched?: boolean;
  provider_switch?: ProviderSwitchInfo;
  provider_switch_reset_done?: boolean;
  always_allow?: boolean;
  reasoning?: string;
  show_reasoning?: boolean;
}

import { TranslationService } from '../translation.service';

@Component({
  selector: 'app-chat',
  standalone: true,
  imports: [CommonModule, FormsModule, DiagramRenderer],
  templateUrl: './chat.html',
  styleUrl: './chat.css'
})
export class Chat implements OnInit, OnChanges, AfterViewChecked {
  private api = inject(ApiService);
  public translation = inject(TranslationService);
  private abortController: AbortController | null = null;

  t(key: string, params?: Record<string, string>): string {
    return this.translation.t(key, params);
  }

  @Input() session: Session | null | undefined = null;
  @Output() agentChanged = new EventEmitter<void>();
  @Output() sessionCreated = new EventEmitter<number>();

  messages = signal<ChatMessageUI[]>([]);
  inputText = signal('');
  isThinking = signal(false);
  isUploading = signal(false);

  agents = signal<Agent[]>([]);
  modelGroups = signal<ModelGroup[]>([]);
  selectedModelId = signal<string>('');
  localAgentId = signal<number | null>(null);

  // Smart Context Cache state
  sessionGoals = signal<string>('');
  workspacePath = signal<string>('');
  sessionFiles = signal<any[]>([]);
  showCacheSettings = signal<boolean>(false);
  isSavingGoals = signal<boolean>(false);
  isSavingWorkspace = signal<boolean>(false);

  // Folder browser state
  showFolderBrowser = signal<boolean>(false);
  browsedPath = signal<string>('');
  browsedDirectories = signal<string[]>([]);
  browsedParentPath = signal<string>('');
  isLoadingDirectories = signal<boolean>(false);

  selectedAgentId = computed(() => {
    return this.session ? this.session.agent_id : this.localAgentId();
  });

  artifacts = signal<any[]>([]);
  activeArtifact = signal<any | null>(null);

  isTextArtifact(art: any): boolean {
    const t = (art?.type || '').toLowerCase();
    const f = (art?.format || '').toLowerCase();
    if (t === 'diagram' || t === 'mermaid') return false;
    const textTypes = ['csv', 'markdown', 'md', 'json', 'text', 'txt', 'report', 'table', 'document'];
    return textTypes.some(k => t.includes(k) || f.includes(k)) || t === '';
  }

  downloadArtifact(art: any): void {
    const f = (art?.format || art?.type || 'txt').toLowerCase();
    const ext = f.includes('csv') ? 'csv' : f.includes('json') ? 'json' : f.includes('markdown') || f === 'md' ? 'md' : 'txt';
    const blob = new Blob([art?.content || ''], {type: 'text/plain;charset=utf-8'});
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${(art?.title || 'artifact').replace(/[^\w\-áéíóúñ ]+/gi, '').trim() || 'artifact'}.${ext}`;
    a.click();
    URL.revokeObjectURL(url);
  }

  @ViewChild('scrollContainer') private scrollContainer!: ElementRef;

  async ngOnInit() {
    this.agents.set(await this.api.getAgents());
    if (this.agents().length > 0) {
      this.localAgentId.set(this.agents()[0].id);
    }
    try {
      this.modelGroups.set(await this.api.getAllModels());
    } catch {
      this.modelGroups.set([]);
    }
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['session']) {
      const prev = changes['session'].previousValue as Session | null;
      const curr = changes['session'].currentValue as Session | null;
      if (curr?.id && curr.id !== prev?.id) {
        this.loadHistory();
        this.loadCacheContext();
      } else if (!curr?.id) {
        this.messages.set([]);
        this.artifacts.set([]);
        this.activeArtifact.set(null);
        this.sessionFiles.set([]);
        this.sessionGoals.set('');
        this.workspacePath.set('');
      }
    }
  }

  async loadCacheContext() {
    if (!this.session?.id) return;
    const s = this.session as any;
    this.sessionGoals.set(s.session_goals || '');
    this.workspacePath.set(s.workspace_path || '');
    try {
      const files = await this.api.getSessionFiles(this.session.id);
      this.sessionFiles.set(files);
    } catch (e) {
      console.error('Failed to load session context files:', e);
    }
  }

  async saveGoals() {
    if (!this.session?.id) return;
    this.isSavingGoals.set(true);
    try {
      await this.api.updateSessionGoals(this.session.id, this.sessionGoals());
      if (this.session) {
        (this.session as any).session_goals = this.sessionGoals();
      }
    } catch (e) {
      console.error('Failed to save goals', e);
    } finally {
      this.isSavingGoals.set(false);
    }
  }

  async saveWorkspace() {
    if (!this.session?.id) return;
    this.isSavingWorkspace.set(true);
    try {
      await this.api.updateSessionWorkspace(this.session.id, this.workspacePath());
      if (this.session) {
        (this.session as any).workspace_path = this.workspacePath();
      }
    } catch (e) {
      console.error('Failed to save workspace path', e);
    } finally {
      this.isSavingWorkspace.set(false);
    }
  }

  async openFolderBrowser() {
    this.showFolderBrowser.set(true);
    await this.browseToPath(this.workspacePath() || '');
  }

  async browseToPath(path: string) {
    this.isLoadingDirectories.set(true);
    try {
      const data = await this.api.browseWorkspace(path);
      this.browsedPath.set(data.current_path);
      this.browsedParentPath.set(data.parent_path);
      this.browsedDirectories.set(data.directories || []);
    } catch (e) {
      console.error('Failed to browse path:', e);
    } finally {
      this.isLoadingDirectories.set(false);
    }
  }

  async selectFolder(folderName: string) {
    const current = this.browsedPath();
    const separator = current.includes('\\') ? '\\' : '/';
    let targetPath = current;
    if (current.endsWith(separator)) {
      targetPath = current + folderName;
    } else {
      targetPath = current + separator + folderName;
    }
    await this.browseToPath(targetPath);
  }

  async goUpFolder() {
    const parent = this.browsedParentPath();
    if (parent && parent !== this.browsedPath()) {
      await this.browseToPath(parent);
    }
  }

  confirmFolderSelection() {
    this.workspacePath.set(this.browsedPath());
    this.showFolderBrowser.set(false);
  }

  async uploadContextFile(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files || input.files.length === 0 || !this.session?.id) return;
    const file = input.files[0];
    this.isUploading.set(true);
    try {
      await this.api.uploadSessionFile(this.session.id, file);
      await this.loadCacheContext();
    } catch (e) {
      console.error('Failed to upload context file', e);
    } finally {
      this.isUploading.set(false);
      input.value = '';
    }
  }

  async deleteContextFile(fileId: number) {
    if (!this.session?.id) return;
    try {
      await this.api.deleteSessionFile(this.session.id, fileId);
      await this.loadCacheContext();
    } catch (e) {
      console.error('Failed to delete context file', e);
    }
  }

  async loadHistory() {
    if (!this.session?.id) return;
    const history = await this.api.getChatHistory(this.session.id);
    this.messages.set(history);
    this.scrollToBottom();

    try {
      const arts = await this.api.getSessionArtifacts(this.session.id);
      this.artifacts.set(arts);
      if (arts.length > 0) {
        this.activeArtifact.set(arts[arts.length - 1]);
      } else {
        this.activeArtifact.set(null);
      }
    } catch (e) {
      console.error('Failed to load session artifacts:', e);
    }
  }

  async onAgentChange(newAgentId: number) {
    if (!this.session) {
      this.localAgentId.set(newAgentId);
      return;
    }
    try {
      await this.api.updateSessionAgent(this.session.id, newAgentId);
      this.agentChanged.emit();
    } catch (e) {
      console.error('Failed to change agent', e);
    }
  }

  selectedModelDisplay(): string {
    const id = this.selectedModelId();
    if (!id) return this.t('chat.default_agent');
    for (const group of this.modelGroups()) {
      for (const m of group.models) {
        if (m.model_id === id) {
          return `${m.name} (${group.provider.name})`;
        }
      }
    }
    return id;
  }

  ngAfterViewChecked() {
    // Only smooth scroll explicitly when sending msgs, here we let natural flow unless forced.
  }

  private scrollToBottom(): void {
    setTimeout(() => {
      try {
        this.scrollContainer.nativeElement.scrollTop = this.scrollContainer.nativeElement.scrollHeight;
      } catch (err) { }
    }, 0);
  }

  async sendMessage() {
    const text = this.inputText().trim();
    if (!text) return;

    let currentSessionId = this.session?.id;
    if (!currentSessionId) {
      this.isThinking.set(true);
      try {
        const newSession = await this.api.createSession();
        currentSessionId = newSession.id;

        // If a specific agent was selected locally, set it
        const agentId = this.localAgentId();
        if (agentId) {
          try {
            await this.api.updateSessionAgent(currentSessionId, agentId);
            newSession.agent_id = agentId;
          } catch (e) {
            console.error('Failed to set initial agent on new session', e);
          }
        }

        this.session = newSession;
        this.sessionCreated.emit(currentSessionId);
      } catch (e) {
        console.error('Failed to auto-create session', e);
        this.isThinking.set(false);
        const detail = e instanceof Error ? e.message : this.t('global.unknown_error');
        this.messages.update(m => [...m, {
          id: Date.now(),
          role: 'system',
          content: `❌ ${this.t('chat.err_auto_start', { error: detail })}`,
          created_at: new Date().toISOString()
        }]);
        return;
      }
    }

    // Optimistic UI updates
    const tempMsg: ChatMessageUI = {
      id: Date.now(),
      role: 'user',
      content: text,
      created_at: new Date().toISOString()
    };

    this.messages.update(m => [...m, tempMsg]);
    this.inputText.set('');
    this.isThinking.set(true);
    this.scrollToBottom();

    const assistantMsgId = Date.now() + 1;
    const assistantMsg: ChatMessageUI = {
      id: assistantMsgId,
      role: 'assistant',
      content: '',
      created_at: new Date().toISOString(),
      tool_calls: []
    };
    this.messages.update(m => [...m, assistantMsg]);

    this.abortController = new AbortController();
    const signal = this.abortController.signal;

    try {
      await this.api.sendChatMessageStream(
        currentSessionId,
        text,
        this.selectedModelId() || undefined,
        (event, data) => {
          if (event === 'token' && data?.text) {
            this.messages.update(list =>
              list.map(msg =>
                msg.id === assistantMsgId
                  ? { ...msg, content: msg.content + data.text }
                  : msg
              )
            );
            this.scrollToBottom();
          } else if (event === 'reasoning' && data?.text) {
            this.messages.update(list =>
              list.map(msg =>
                msg.id === assistantMsgId
                  ? { ...msg, reasoning: (msg.reasoning || '') + data.text }
                  : msg
              )
            );
            this.scrollToBottom();
          } else if (event === 'tool_start' && data?.name) {
            this.messages.update(list =>
              list.map(msg =>
                msg.id === assistantMsgId
                  ? { ...msg, content: msg.content + `\n\n*🔧 ${this.t('chat.executing', { name: data.name })}...*` }
                  : msg
              )
            );
            this.scrollToBottom();
          } else if (event === 'tool_end' && data?.name) {
            this.messages.update(list =>
              list.map(msg =>
                msg.id === assistantMsgId
                  ? { ...msg, content: msg.content + `\n\n*✅ ${this.t('chat.completed', { name: data.name })}.*` }
                  : msg
              )
            );
            this.scrollToBottom();
          } else if (event === 'provider_switch' && data) {
            const switchInfo = data as ProviderSwitchInfo;
            this.messages.update(list =>
              list.map(msg =>
                msg.id === assistantMsgId
                  ? {
                      ...msg,
                      provider_switched: true,
                      provider_switch: switchInfo
                    }
                  : msg
              )
            );
          } else if (event === 'confirmation_required' && data) {
            this.messages.update(list =>
              list.map(msg =>
                msg.id === assistantMsgId
                  ? {
                      ...msg,
                      requires_confirmation: true,
                      pending_action_id: data.pending_action_id,
                      waiting_tool: data.waiting_tool
                    }
                  : msg
              )
            );
            this.scrollToBottom();
          } else if (event === 'error' && data?.message) {
            this.messages.update(list =>
              list.map(msg =>
                msg.id === assistantMsgId
                  ? { ...msg, content: msg.content + `\n\n*❌ Error: ${data.message}*` }
                  : msg
              )
            );
          }
        },
        signal
      );

      // Sincronizar artefactos finales
      const arts = await this.api.getSessionArtifacts(currentSessionId);
      this.artifacts.set(arts);
      if (arts.length > 0) {
        this.activeArtifact.set(arts[arts.length - 1]);
      }
      this.scrollToBottom();
    } catch (e) {
      if (e instanceof Error && e.name === 'AbortError') {
        console.log('Stream generation aborted by user');
        return;
      }
      console.error(e);
      const detail = e instanceof Error ? e.message : this.t('global.unknown_error');
      // Limpiar mensaje del asistente si quedó vacío por error inicial
      this.messages.update(list => list.filter(msg => msg.id !== assistantMsgId));
      this.messages.update(m => [...m, {
        id: Date.now() + 2,
        role: 'system',
        content: `❌ ${this.t('global.error')}: ${detail}`,
        created_at: new Date().toISOString()
      }]);
      this.scrollToBottom();
    } finally {
      this.abortController = null;
      this.isThinking.set(false);
    }
  }

  stopGeneration() {
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }
    this.isThinking.set(false);

    // Remove the last assistant message from UI
    this.messages.update(list => {
      if (list.length > 0 && list[list.length - 1].role === 'assistant') {
        return list.slice(0, list.length - 1);
      }
      return list;
    });
  }

  async editMessage(msg: ChatMessageUI) {
    if (!this.session?.id || this.isThinking()) return;

    // 1. Copy message content to input box
    this.inputText.set(msg.content);

    // 2. Call backend to delete all messages from this message onwards
    try {
      await this.api.deleteChatMessagesFrom(this.session.id, msg.id);

      // 3. Remove them from the frontend state
      const idx = this.messages().findIndex(m => m.id === msg.id);
      if (idx !== -1) {
        this.messages.set(this.messages().slice(0, idx));
      }

      // Reload artifacts since history changed
      const arts = await this.api.getSessionArtifacts(this.session.id);
      this.artifacts.set(arts);
      if (arts.length > 0) {
        this.activeArtifact.set(arts[arts.length - 1]);
      } else {
        this.activeArtifact.set(null);
      }
    } catch (e) {
      console.error('Failed to edit/truncate message:', e);
    }
  }

  async resetProviderOverride(msg: ChatMessageUI) {
    if (!this.session?.id || this.isThinking() || msg.provider_switch_reset_done) return;
    this.isThinking.set(true);
    try {
      await this.api.resetSessionLLMOverride(this.session.id);
      msg.provider_switch_reset_done = true;
      this.messages.update(m => [...m, {
        id: Date.now(),
        role: 'system',
        content: '✅ ' + this.t('chat.provider_restored_msg'),
        created_at: new Date().toISOString()
      }]);
      this.scrollToBottom();
    } catch (e: any) {
      this.messages.update(m => [...m, {
        id: Date.now(),
        role: 'system',
        content: `❌ ${this.t('chat.provider_restore_error_msg')}: ${e?.message || this.t('global.unknown_error')}`,
        created_at: new Date().toISOString()
      }]);
      this.scrollToBottom();
    } finally {
      this.isThinking.set(false);
    }
  }

  async approveAction(msg: ChatMessageUI) {
    if (this.isThinking() || !this.session?.id || !msg.pending_action_id) return;

    this.isThinking.set(true);
    try {
      const res = await this.api.confirmAction(this.session.id, msg.pending_action_id, true, !!msg.always_allow);
      if (res?.status === 'error') {
        msg.requires_confirmation = false;
        await this.loadHistory();
        return;
      }
      msg.confirmed = true;
      msg.requires_confirmation = false;

      if (res?.artifacts && res.artifacts.length > 0) {
        this.artifacts.update(current => {
          const updated = [...current];
          for (const art of res.artifacts!) {
            const idx = updated.findIndex(a => a.id === art.id);
            if (idx > -1) {
              updated[idx] = art;
            } else {
              updated.push(art);
            }
          }
          return updated;
        });
        this.activeArtifact.set(res.artifacts[res.artifacts.length - 1]);
      }

      await this.loadHistory();
    } catch (e: any) {
      console.error(e);
      msg.requires_confirmation = false;
      this.messages.update(m => [...m, {
        id: Date.now(),
        role: 'system',
        content: `❌ ${this.t('chat.action_error')}: ${e.message}`,
        created_at: new Date().toISOString()
      }]);
      this.scrollToBottom();
    } finally {
      this.isThinking.set(false);
    }
  }

  async rejectAction(msg: ChatMessageUI) {
    if (this.isThinking() || !this.session?.id || !msg.pending_action_id) return;

    this.isThinking.set(true);
    try {
      await this.api.confirmAction(this.session.id, msg.pending_action_id, false);
      msg.rejected = true;
      msg.requires_confirmation = false;
      await this.loadHistory();
    } catch (e) {
      console.error(e);
    } finally {
      this.isThinking.set(false);
    }
  }

  reopenCanvas() {
    const arts = this.artifacts();
    if (arts.length > 0) {
      this.activeArtifact.set(arts[arts.length - 1]);
    }
  }

  async onNodeClicked(nodeLabel: string) {
    const text = this.t('chat.node_clicked_prompt', { node: nodeLabel });
    this.inputText.set(text);
    await this.sendMessage();
  }

  async onFileSelected(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files || input.files.length === 0) return;

    const file = input.files[0];
    this.isUploading.set(true);

    const uploadingText = this.t('chat.uploading_placeholder');

    // Add optimist loading system message
    const tempMsgId = Date.now();
    this.messages.update(m => [...m, {
      id: tempMsgId,
      role: 'system',
      content: `📎 ${uploadingText}`,
      created_at: new Date().toISOString()
    }]);
    this.scrollToBottom();

    try {
      // Perform upload with standard chunking configurations
      const res = await this.api.uploadKnowledgeFile(file, 500, 50);
      
      // Update loading message with success confirmation
      this.messages.update(list => 
        list.map(msg => 
          msg.id === tempMsgId
            ? { 
                ...msg, 
                content: `✅ ${this.t('chat.upload_success', { name: file.name, chunks: res.chunks.toString() })}` 
              }
            : msg
        )
      );
    } catch (e: any) {
      console.error(e);
      const detail = e?.message || this.t('global.unknown_error');
      // Update loading message with error description
      this.messages.update(list => 
        list.map(msg => 
          msg.id === tempMsgId
            ? { 
                ...msg, 
                content: `❌ ${this.t('chat.upload_error', { error: detail })}` 
              }
            : msg
        )
      );
    } finally {
      this.isUploading.set(false);
      input.value = ''; // Clear file input selection
      this.scrollToBottom();
    }
  }

  onKeyDown(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      this.sendMessage();
    }
  }
}
