import { Component, signal, inject, OnInit, computed } from '@angular/core';
import { ApiService, Agent, LLMProvider } from '../api.service';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-agents',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './agents.html',
  styleUrl: './agents.css'
})
export class AgentsComponent implements OnInit {
  private api = inject(ApiService);
  
  agents = signal<Agent[]>([]);
  providers = signal<LLMProvider[]>([]);
  availableTools = signal<any[]>([]);
  toolSearchQuery = signal('');

  filteredTools = computed(() => {
    const query = this.toolSearchQuery().toLowerCase();
    const tools = this.availableTools();
    if (!query) return tools;
    return tools.filter(t => t.name.toLowerCase().includes(query));
  });
  
  showModal = signal(false);
  isEditing = signal(false);
  
  // Form Model
  currentAgent = {
    id: 0,
    name: '',
    description: '',
    llm_provider_id: 0,
    tools: [] as string[]
  };

  async ngOnInit() {
    this.loadData();
  }

  async loadData() {
    try {
      const [aResult, pResult, tResult] = await Promise.allSettled([
        this.api.getAgents(),
        this.api.getProviders(),
        this.api.getActiveTools()
      ]);
      
      if (aResult.status === 'fulfilled') {
        this.agents.set(aResult.value);
      } else {
        console.warn('Failed to load agents list:', aResult.reason);
      }

      if (pResult.status === 'fulfilled') {
        this.providers.set(pResult.value);
      }
      
      if (tResult.status === 'fulfilled') {
        this.availableTools.set(tResult.value);
      }
      
      if (!this.isEditing() && this.providers().length > 0) {
        const defaultProvider = this.providers().find(p => p.is_default) || this.providers()[0];
        this.currentAgent.llm_provider_id = defaultProvider.id;
      }
    } catch (err) {
      console.error('Unexpected error loading agents data:', err);
    }
  }

  openCreateModal() {
    this.isEditing.set(false);
    this.toolSearchQuery.set('');
    this.currentAgent = { 
      id: 0, 
      name: '', 
      description: '',
      llm_provider_id: this.providers().find(p => p.is_default)?.id || this.providers()[0]?.id || 0, 
      tools: [] 
    };
    this.showModal.set(true);
  }

  async openEditModal(agent: Agent) {
    this.isEditing.set(true);
    this.toolSearchQuery.set('');
    try {
      const full = await this.api.getAgent(agent.id);
      this.currentAgent = {
        id: full.id,
        name: full.name,
        description: full.description,
        llm_provider_id: full.llm_provider_id,
        tools: full.tools?.map(t => t.tool_name) || []
      };
      this.showModal.set(true);
    } catch (e) {
      console.error('No se pudo cargar el agente:', e);
      alert('No se pudo cargar la configuración del agente.');
      this.isEditing.set(false);
    }
  }

  toggleTool(toolName: string) {
    const idx = this.currentAgent.tools.indexOf(toolName);
    if (idx > -1) {
      this.currentAgent.tools.splice(idx, 1);
    } else {
      this.currentAgent.tools.push(toolName);
    }
  }

  isToolSelected(toolName: string): boolean {
    return this.currentAgent.tools.includes(toolName);
  }

  selectAllTools() {
    this.currentAgent.tools = this.availableTools().map(t => t.name);
  }

  deselectAllTools() {
    this.currentAgent.tools = [];
  }

  async saveAgent() {
    try {
      if (this.isEditing()) {
        await this.api.updateAgent(this.currentAgent.id, this.currentAgent);
      } else {
        await this.api.createAgent(this.currentAgent);
      }
      this.showModal.set(false);
      this.loadData();
    } catch (err) {
      alert('Error al guardar el agente: ' + err);
    }
  }

  async deleteAgent(id: number) {
    if (id === 1) return; // Protect General
    if (confirm('¿Seguro que querés eliminar este agente?')) {
      try {
        await this.api.deleteAgent(id);
        this.loadData();
      } catch (err) {
        alert('Error al eliminar el agente: ' + err);
      }
    }
  }
}
