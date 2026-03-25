import { Component, signal, inject, OnInit } from '@angular/core';
import { ApiService, Rule } from '../api.service';
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
  
  newCategory = signal('');
  newContent = signal('');

  async ngOnInit() {
    this.loadRules();
  }

  async loadRules() {
    const r = await this.api.getRules();
    this.rules.set(r);
  }

  async createRule() {
    if (!this.newCategory() || !this.newContent()) return;
    
    await this.api.createRule({
      category: this.newCategory(),
      content: this.newContent(),
      importance: 1
    });
    
    this.newCategory.set('');
    this.newContent.set('');
    this.loadRules();
  }

  async deleteRule(id: number) {
    await this.api.deleteRule(id);
    this.loadRules();
  }
}
