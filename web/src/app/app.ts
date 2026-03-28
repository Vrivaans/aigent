import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Chat } from './chat/chat';
import { Dashboard } from './dashboard/dashboard';
import { RuleConfig } from './rule-config/rule-config';
import { Providers } from './providers/providers';
import { ApiService, Session } from './api.service';
import { AuthService } from './auth/auth.service';
import { LoginComponent } from './auth/login';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [CommonModule, Chat, Dashboard, RuleConfig, Providers, LoginComponent],
  templateUrl: './app.html',
  styleUrl: './app.css'
})
export class App implements OnInit {
  private api = inject(ApiService);
  public auth = inject(AuthService);
  
  currentTab: 'chats' | 'dashboard' | 'rules' | 'tools' | 'providers' = 'chats';
  sessions = signal<Session[]>([]);
  activeSessionId = signal<number | null>(null);
  tools = signal<any[]>([]);
  toolsLoading = signal(false);

  async ngOnInit() {
    if (this.auth.isLoggedIn()) {
      await this.initAppData();
    }
  }

  async initAppData() {
    await this.loadSessions();
    await this.loadTools();
    if (this.sessions().length > 0) {
      this.activeSessionId.set(this.sessions()[0].id);
    } else {
      await this.createNewSession();
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
      await this.loadTools();
    } finally {
      this.toolsLoading.set(false);
    }
  }

  async loadSessions() {
    const s = await this.api.getSessions();
    this.sessions.set(s);
  }

  async createNewSession() {
    const newSession = await this.api.createSession();
    await this.loadSessions();
    this.activeSessionId.set(newSession.id);
    this.currentTab = 'chats';
  }

  selectSession(id: number) {
    this.activeSessionId.set(id);
    this.currentTab = 'chats';
  }
}

