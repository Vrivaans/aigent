import { Component, signal, inject, OnInit, ViewChild, ElementRef, AfterViewChecked, Input, Output, EventEmitter, OnChanges, SimpleChanges } from '@angular/core';
import { ApiService, ChatMessage, Session, Agent, ProviderSwitchInfo } from '../api.service';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

export interface ChatMessageUI extends ChatMessage {
  requires_confirmation?: boolean;
  pending_action_id?: number;
  waiting_tool?: any;
  confirmed?: boolean;
  rejected?: boolean;
  provider_switched?: boolean;
  provider_switch?: ProviderSwitchInfo;
  provider_switch_reset_done?: boolean;
}

@Component({
  selector: 'app-chat',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './chat.html',
  styleUrl: './chat.css'
})
export class Chat implements OnInit, OnChanges, AfterViewChecked {
  private api = inject(ApiService);
  
  @Input({ required: true }) session!: Session;
  @Output() agentChanged = new EventEmitter<void>();
  
  messages = signal<ChatMessageUI[]>([]);
  inputText = signal('');
  isThinking = signal(false);
  
  agents = signal<Agent[]>([]);

  @ViewChild('scrollContainer') private scrollContainer!: ElementRef;

  async ngOnInit() {
    this.agents.set(await this.api.getAgents());
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['session'] && this.session?.id) {
      this.loadHistory();
    }
  }

  async loadHistory() {
    if (!this.session?.id) return;
    const history = await this.api.getChatHistory(this.session.id);
    this.messages.set(history);
    this.scrollToBottom();
  }

  async onAgentChange(newAgentId: number) {
    if (!this.session?.id) return;
    try {
      await this.api.updateSessionAgent(this.session.id, newAgentId);
      this.agentChanged.emit();
    } catch (e) {
      console.error('Failed to change agent', e);
    }
  }

  ngAfterViewChecked() {
    // Only smooth scroll explicitly when sending msgs, here we let natural flow unless forced.
  }

  private scrollToBottom(): void {
    setTimeout(() => {
      try {
        this.scrollContainer.nativeElement.scrollTop = this.scrollContainer.nativeElement.scrollHeight;
      } catch(err) { }
    }, 0);
  }

  async sendMessage() {
    if (!this.session?.id) return;
    const text = this.inputText().trim();
    if (!text) return;

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
      const res = await this.api.sendChatMessage(this.session.id, text);
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
      const res = await this.api.confirmAction(this.session.id, msg.pending_action_id, true);
      if (res?.status === 'error') {
        msg.requires_confirmation = false;
        await this.loadHistory();
        return;
      }
      msg.confirmed = true;
      msg.requires_confirmation = false;
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

  onKeyDown(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      this.sendMessage();
    }
  }
}
