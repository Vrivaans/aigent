import { Component, signal, inject, OnInit, computed } from '@angular/core';
import { ApiService, Rule, Agent } from '../api.service';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-rule-config',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './rule-config.html',
  styleUrl: './rule-config.css'
})
export class RuleConfig implements OnInit {
  private api = inject(ApiService);
  rules = signal<Rule[]>([]);
  agents = signal<Agent[]>([]);
  
  newCategory = signal('');
  newContent = signal('');
  selectedAgentIds = signal<Set<number>>(new Set());

  isAgentSelected = computed(() => (id: number) => this.selectedAgentIds().has(id));

  async ngOnInit() {
    this.loadRules();
    this.loadAgents();
  }

  async loadRules() {
    const r = await this.api.getRules();
    this.rules.set(r);
  }

  async loadAgents() {
    const a = await this.api.getAgents();
    this.agents.set(a);
  }

  toggleAgent(id: number) {
    const current = new Set(this.selectedAgentIds());
    if (current.has(id)) {
      current.delete(id);
    } else {
      current.add(id);
    }
    this.selectedAgentIds.set(current);
  }

  getAgentNames(rule: Rule): string {
    return rule.agents?.map(a => a.name).join(', ') || '';
  }

  async createRule() {
    if (!this.newCategory() || !this.newContent()) return;
    
    const agentIds = Array.from(this.selectedAgentIds());

    await this.api.createRule({
      agent_ids: agentIds,
      category: this.newCategory(),
      content: this.newContent(),
      importance: 1
    } as any);
    
    this.newCategory.set('');
    this.newContent.set('');
    this.selectedAgentIds.set(new Set());
    this.loadRules();
  }

  async deleteRule(id: number) {
    await this.api.deleteRule(id);
    this.loadRules();
  }
}
