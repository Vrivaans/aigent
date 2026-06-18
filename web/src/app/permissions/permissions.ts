import { Component, signal, inject, OnInit } from '@angular/core';
import { ApiService, Agent, ApprovalPolicy } from '../api.service';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TranslationService } from '../translation.service';
import { AuthService } from '../auth/auth.service';

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
  auth = inject(AuthService);

  t(key: string, params?: Record<string, string>): string {
    return this.translation.t(key, params);
  }
  
  permissions = signal<ToolPermission[]>([]);
  agents = signal<Agent[]>([]);
  policies = signal<ApprovalPolicy[]>([]);
  isLoading = signal(false);
  isSavingPolicy = signal(false);

  newPolicyPattern = '';
  newPolicyEnvironment = '*';
  newPolicyMinRole = 'operator';

  async ngOnInit() {
    await this.loadData();
  }

  async loadData() {
    this.isLoading.set(true);
    try {
      const requests: Promise<unknown>[] = [
        this.api.getPermissions(),
        this.api.getAgents()
      ];
      if (this.auth.hasRole('admin')) {
        requests.push(this.api.getApprovalPolicies());
      }
      const results = await Promise.all(requests);
      this.permissions.set(results[0] as ToolPermission[]);
      this.agents.set(results[1] as Agent[]);
      if (this.auth.hasRole('admin') && results[2]) {
        this.policies.set(results[2] as ApprovalPolicy[]);
      }
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

  async createPolicy() {
    const pattern = this.newPolicyPattern.trim();
    if (!pattern) {
      alert(this.t('permissions.policy_pattern_required'));
      return;
    }
    this.isSavingPolicy.set(true);
    try {
      const policy = await this.api.createApprovalPolicy({
        tool_pattern: pattern,
        environment: this.newPolicyEnvironment.trim() || '*',
        min_role: this.newPolicyMinRole.trim() || 'operator',
        requires_approval: true
      });
      this.policies.update(list => [...list, policy]);
      this.newPolicyPattern = '';
      this.newPolicyEnvironment = '*';
      this.newPolicyMinRole = 'operator';
    } catch (err) {
      console.error('Failed to create approval policy:', err);
      alert(this.t('permissions.policy_error_create'));
    } finally {
      this.isSavingPolicy.set(false);
    }
  }

  async deletePolicy(id: number) {
    if (!confirm(this.t('permissions.policy_delete_confirm'))) {
      return;
    }
    try {
      await this.api.deleteApprovalPolicy(id);
      this.policies.update(list => list.filter(p => p.id !== id));
    } catch (err) {
      console.error('Failed to delete approval policy:', err);
      alert(this.t('permissions.policy_error_delete'));
    }
  }
}
