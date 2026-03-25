import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, LLMProvider } from '../api.service';

@Component({
  selector: 'app-providers',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './providers.html',
  styleUrl: './providers.css'
})
export class Providers implements OnInit {
  private api = inject(ApiService);
  
  providers = signal<LLMProvider[]>([]);
  showAddForm = signal(false);
  isEditing = signal(false);
  editingId = signal<number | null>(null);
  testResult = signal<{ ok: boolean; message: string } | null>(null);
  isTesting = signal(false);
  
  newProvider: Partial<LLMProvider> = {
    name: '',
    base_url: 'https://api.groq.com/openai/v1',
    api_key: '',
    default_model: 'llama-3.3-70b-versatile'
  };

  async ngOnInit() {
    await this.loadProviders();
  }

  async loadProviders() {
    const p = await this.api.getProviders();
    this.providers.set(p);
  }

  async onSaveProvider() {
    if (!this.newProvider.name) return;
    
    if (this.isEditing() && this.editingId()) {
      await this.api.updateProvider(this.editingId()!, this.newProvider);
    } else {
      await this.api.createProvider(this.newProvider);
    }

    await this.loadProviders();
    this.showAddForm.set(false);
    this.resetForm();
  }

  editProvider(p: LLMProvider) {
    this.isEditing.set(true);
    this.editingId.set(p.ID);
    this.newProvider = {
      name: p.name,
      base_url: p.base_url,
      api_key: '********', // Placeholder to indicate we have a key
      default_model: p.default_model
    };
    this.showAddForm.set(true);
  }

  resetForm() {
    this.isEditing.set(false);
    this.editingId.set(null);
    this.newProvider = {
      name: '',
      base_url: 'https://api.groq.com/openai/v1',
      api_key: '',
      default_model: 'llama-3.3-70b-versatile'
    };
  }

  async setAsDefault(id: number, event: Event) {
    event.stopPropagation();
    await this.api.setDefaultProvider(id);
    await this.loadProviders();
  }

  async deleteProvider(id: number, event: Event) {
    event.stopPropagation();
    if (!confirm('¿Eliminar este proveedor?')) return;
    await this.api.deleteProvider(id);
    await this.loadProviders();
  }

  async testProvider() {
    const id = this.editingId();
    if (!id) return;
    this.isTesting.set(true);
    this.testResult.set(null);
    try {
      const res = await this.api.testProvider(id);
      this.testResult.set(res);
    } catch (e: any) {
      this.testResult.set({ ok: false, message: e.message || 'Error desconocido' });
    } finally {
      this.isTesting.set(false);
    }
  }
}
