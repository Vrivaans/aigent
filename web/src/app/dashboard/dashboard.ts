import { Component, signal, inject, OnInit } from '@angular/core';
import { ApiService, Task } from '../api.service';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.css'
})
export class Dashboard implements OnInit {
  private api = inject(ApiService);
  tasks = signal<Task[]>([]);
  tools = signal<any[]>([]);
  isSaving = signal(false);
  error = signal('');

  newTask = {
    name: '',
    cron_expression: '@hourly',
    tool_name: '',
    payload: '{}'
  };

  async ngOnInit() {
    this.loadTasks();
    this.loadTools();
    setInterval(() => this.loadTasks(), 15000); // Live reload MVP
  }

  async loadTasks() {
    const t = await this.api.getTasks();
    this.tasks.set(t);
  }

  async loadTools() {
    const tools = await this.api.getActiveTools();
    const schedulableTools = tools.filter((tool) => tool.name !== 'schedule_task');
    this.tools.set(schedulableTools);
    if (!this.newTask.tool_name && schedulableTools.length > 0) {
      this.newTask.tool_name = schedulableTools[0].name;
    }
  }

  async createTask() {
    this.error.set('');

    let payload: Record<string, any>;
    try {
      payload = JSON.parse(this.newTask.payload || '{}');
    } catch {
      this.error.set('El payload debe ser JSON válido.');
      return;
    }

    if (!payload || Array.isArray(payload) || typeof payload !== 'object') {
      this.error.set('El payload debe ser un objeto JSON.');
      return;
    }

    this.isSaving.set(true);
    try {
      await this.api.createTask({
        name: this.newTask.name,
        cron_expression: this.newTask.cron_expression,
        tool_name: this.newTask.tool_name,
        payload
      });
      this.newTask.name = '';
      this.newTask.payload = '{}';
      await this.loadTasks();
    } catch (err) {
      this.error.set(err instanceof Error ? err.message : 'No se pudo crear la tarea.');
    } finally {
      this.isSaving.set(false);
    }
  }

  async deleteTask(id: number) {
    await this.api.deleteTask(id);
    this.loadTasks();
  }

  formatDate(d?: string | null): string {
    if (!d) return 'Pendiente';
    return new Date(d).toLocaleString();
  }
}
