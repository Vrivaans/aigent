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
}

@Component({
  selector: 'app-chat',
  standalone: true,
  imports: [CommonModule, FormsModule, DiagramRenderer],
  templateUrl: './chat.html',
  styleUrl: './chat.css'
})
export class Chat implements OnInit, OnChanges, AfterViewChecked {
  private api = inject(ApiService);

  @Input() session: Session | null | undefined = null;
  @Output() agentChanged = new EventEmitter<void>();
  @Output() sessionCreated = new EventEmitter<number>();

  messages = signal<ChatMessageUI[]>([]);
  inputText = signal('');
  isThinking = signal(false);

  agents = signal<Agent[]>([]);
  modelGroups = signal<ModelGroup[]>([]);
  selectedModelId = signal<string>('');
  localAgentId = signal<number | null>(null);

  selectedAgentId = computed(() => {
    return this.session ? this.session.agent_id : this.localAgentId();
  });

  artifacts = signal<any[]>([]);
  activeArtifact = signal<any | null>(null);

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
      } else if (!curr?.id) {
        this.messages.set([]);
        this.artifacts.set([]);
        this.activeArtifact.set(null);
      }
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
    if (!id) return 'Default model';
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
        const detail = e instanceof Error ? e.message : 'Error desconocido';
        this.messages.update(m => [...m, {
          id: Date.now(),
          role: 'system',
          content: `❌ Error al iniciar conversación automáticamente: ${detail}`,
          created_at: new Date().toISOString()
        }]);
        return;
      }
    }

    // Optimistic UI updates
    const tempMsg: ChatMessage = {
      id: Date.now(),
      role: 'user',
      content: text,
      created_at: new Date().toISOString()
    };

    this.messages.update(m => [...m, tempMsg]);
    this.inputText.set('');
    this.isThinking.set(true);
    this.scrollToBottom();

    try {
      const res = await this.api.sendChatMessage(currentSessionId, text, this.selectedModelId() || undefined);
      if (res.status === 'error') {
        await this.loadHistory();
        return;
      }
      this.messages.update(m => [...m, {
        id: Date.now() + 1,
        role: 'assistant',
        content: res.response,
        created_at: new Date().toISOString(),
        tool_calls: res.tool_calls,
        requires_confirmation: res.requires_confirmation,
        pending_action_id: res.pending_action_id,
        waiting_tool: res.waiting_tool,
        provider_switched: res.provider_switched,
        provider_switch: res.provider_switch
      }]);

      if (res.artifacts && res.artifacts.length > 0) {
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

      this.scrollToBottom();
    } catch (e) {
      console.error(e);
      const detail = e instanceof Error ? e.message : 'Error desconocido';
      this.messages.update(m => [...m, {
        id: Date.now() + 1,
        role: 'system',
        content: `❌ Error: ${detail}`,
        created_at: new Date().toISOString()
      }]);
      this.scrollToBottom();
    } finally {
      this.isThinking.set(false);
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
        content: '✅ Se restauró el provider/modelo default del agente para esta conversación.',
        created_at: new Date().toISOString()
      }]);
      this.scrollToBottom();
    } catch (e: any) {
      this.messages.update(m => [...m, {
        id: Date.now(),
        role: 'system',
        content: `❌ No se pudo restaurar el default del agente: ${e?.message || 'Error desconocido'}`,
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
        content: `❌ Error al ejecutar la acción: ${e.message}`,
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
    const text = `Explicame más sobre el nodo "${nodeLabel}" del diagrama.`;
    this.inputText.set(text);
    await this.sendMessage();
  }

  onKeyDown(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      this.sendMessage();
    }
  }
}
