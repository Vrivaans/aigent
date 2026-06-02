import { Component, signal, inject, OnInit } from '@angular/core';
import { ApiService, Agent } from '../api.service';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TranslationService } from '../translation.service';

export interface ToolPermission {
  id: number;
  agent_id: number;
  tool_name: string;
  action_type: string;
  paused: boolean;
  created_at: string;
  updated_at: string;
}

@Component({
  selector: 'app-permissions',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './permissions.html',
  styleUrl: './permissions.css'
})
export class PermissionsComponent implements OnInit {
  private api = inject(ApiService);
  private translation = inject(TranslationService);

  t(key: string, params?: Record<string, string>): string {
    return this.translation.t(key, params);
  }
  
  permissions = signal<ToolPermission[]>([]);
  agents = signal<Agent[]>([]);
  isLoading = signal(false);

  async ngOnInit() {
    await this.loadData();
  }

  async loadData() {
    this.isLoading.set(true);
    try {
      const [p, a] = await Promise.all([
        this.api.getPermissions(),
        this.api.getAgents()
      ]);
      this.permissions.set(p);
      this.agents.set(a);
    } catch (err) {
      console.error('Failed to load permissions or agents:', err);
    } finally {
      this.isLoading.set(false);
    }
  }

  getDayKey(key: string): string {
    return '';
  }

  getAgentName(agentId: number): string {
    const agent = this.agents().find(a => a.id === agentId);
    return agent ? agent.name : this.t('permissions.agent_label_id', { id: '' + agentId });
  }

  async togglePause(perm: ToolPermission) {
    try {
      const updated = await this.api.togglePausePermission(perm.id);
      this.permissions.update(list => 
        list.map(p => p.id === perm.id ? { ...p, paused: updated.paused } : p)
      );
    } catch (err) {
      console.error('Failed to toggle pause on permission:', err);
      alert(this.t('permissions.error_modify'));
    }
  }

  async revokePermission(id: number) {
    if (!confirm(this.t('permissions.delete_confirm'))) {
      return;
    }
    try {
      await this.api.deletePermission(id);
      this.permissions.update(list => list.filter(p => p.id !== id));
    } catch (err) {
      console.error('Failed to delete permission:', err);
      alert(this.t('permissions.error_delete'));
    }
  }
}
