import { Component, inject, OnInit, signal, computed, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Chat } from './chat/chat';
import { Dashboard } from './dashboard/dashboard';
import { RuleConfig } from './rule-config/rule-config';
import { Providers } from './providers/providers';
import { AgentsComponent } from './agents/agents';
import { ApiService, Session } from './api.service';
import { AuthService } from './auth/auth.service';
import { LoginComponent } from './auth/login';
import { PermissionsComponent } from './permissions/permissions';
import { ApprovalsComponent } from './approvals/approvals';
import { WorkflowsComponent } from './workflows/workflows';
import { UsersComponent } from './users/users';
import { AuditComponent } from './audit/audit';

import { TranslationService } from './translation.service';

export type Tab = 'chats' | 'dashboard' | 'rules' | 'providers' | 'tools' | 'agents' | 'permissions' | 'approvals' | 'workflows' | 'users' | 'audit';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule, Chat, Dashboard, RuleConfig, Providers, LoginComponent,
    AgentsComponent, PermissionsComponent, ApprovalsComponent, WorkflowsComponent, UsersComponent, AuditComponent
  ],
  templateUrl: './app.html',
  styleUrl: './app.css'
})
export class App implements OnInit {
  private api = inject(ApiService);
  public auth = inject(AuthService);
  public translation = inject(TranslationService);

  t(key: string, params?: Record<string, string>): string {
    return this.translation.t(key, params);
  }

  constructor() {
    effect(async () => {
      if (this.auth.isLoggedIn()) {
        await this.initAppData();
      }
    });
  }
  
  currentTab: Tab = 'chats';
  sessions = signal<Session[]>([]);
  activeSessionId = signal<number | null>(null);
  activeSession = computed(() => this.sessions().find(s => s.id === this.activeSessionId()) ?? null);
  showCronSessions = signal<boolean>(false);
  showWorkflowSessions = signal<boolean>(false);

  tools = signal<any[]>([]);
  toolsLoading = signal(false);
  toolSearchQuery = signal('');
  
  filteredTools = computed(() => {
    const query = this.toolSearchQuery().toLowerCase().trim();
    if (!query) return this.tools();
    return this.tools().filter(t => 
      t.name.toLowerCase().includes(query) || 
      t.description.toLowerCase().includes(query)
    );
  });

  isMenuOpen = signal(false);
  isSidebarOpen = signal(true);
  pendingApprovalsCount = signal(0);
  private pollIntervalId: any;

  async ngOnInit() {
    if (this.auth.isLoggedIn()) {
      await this.initAppData();
    }
    // Poll approvals count every 15 seconds
    this.pollIntervalId = setInterval(async () => {
      if (this.auth.isLoggedIn()) {
        await this.loadPendingCount();
      }
    }, 15000);
  }

  async initAppData() {
    await this.loadSessions();
    await this.loadTools();
    await this.loadPendingCount();
    if (this.sessions().length > 0) {
      this.activeSessionId.set(this.sessions()[0].id);
    } else {
      this.activeSessionId.set(null);
    }
  }

  async loadPendingCount() {
    try {
      const list = await this.api.getPendingApprovals();
      this.pendingApprovalsCount.set(list?.length || 0);
    } catch (err) {
      console.error('Failed to load pending approvals count:', err);
    }
  }

  logout() {
    this.auth.logout();
  }

  async loadTools() {
    const t = await this.api.getActiveTools();
    this.tools.set(t ?? []);
  }

  async refreshTools() {
    this.toolsLoading.set(true);
    try {
      const t = await this.api.getActiveTools(true);
      this.tools.set(t ?? []);
    } finally {
      this.toolsLoading.set(false);
    }
  }

  async loadSessions() {
    const s = await this.api.getSessions(!this.showCronSessions(), !this.showWorkflowSessions());
    this.sessions.set(s);
  }

  async toggleCronSessions() {
    this.showCronSessions.update(v => !v);
    await this.loadSessions();
  }

  async toggleWorkflowSessions() {
    this.showWorkflowSessions.update(v => !v);
    await this.loadSessions();
  }

  async createNewSession() {
    const newSession = await this.api.createSession();
    await this.loadSessions();
    this.activeSessionId.set(newSession.id);
    this.currentTab = 'chats';
    this.isMenuOpen.set(false);
    if (window.innerWidth < 768) this.isSidebarOpen.set(false);
  }

  selectSession(id: number) {
    this.activeSessionId.set(id);
    this.currentTab = 'chats';
    this.isMenuOpen.set(false);
    if (window.innerWidth < 768) this.isSidebarOpen.set(false);
  }

  async deleteSession(id: number, event: Event) {
    event.stopPropagation(); // Evitar que se seleccione el chat al borrarlo
    if (!confirm('Seguro que querés eliminar?')) return;

    try {
      await this.api.deleteSession(id);
      await this.loadSessions();

      // Si borramos el activo, seleccionar otro
      if (this.activeSessionId() === id) {
        if (this.sessions().length > 0) {
          this.activeSessionId.set(this.sessions()[0].id);
        } else {
          this.activeSessionId.set(null);
        }
      }
    } catch (err) {
      console.error('Failed to delete session', err);
      alert('Error al eliminar la conversación');
    }
  }

  async handleSessionCreated(sessionId: number) {
    await this.loadSessions();
    this.activeSessionId.set(sessionId);
  }
}

