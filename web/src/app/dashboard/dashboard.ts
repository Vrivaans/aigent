import { Component, signal, inject, OnInit } from '@angular/core';
import { ApiService, Task, Agent } from '../api.service';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

interface DayOption {
  key: string;
  label: string;
}

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
  agents = signal<Agent[]>([]);
  isSaving = signal(false);
  error = signal('');

  days: DayOption[] = [
    { key: '1', label: 'L' },
    { key: '2', label: 'M' },
    { key: '3', label: 'X' },
    { key: '4', label: 'J' },
    { key: '5', label: 'V' },
    { key: '6', label: 'S' },
    { key: '0', label: 'D' },
  ];

  newTask = {
    name: '',
    frequency: '@daily',
    scheduledHour: '09:00',
    customHour: '09:00',
    customDays: [] as string[],
    oneShot: false,
    cron_expression: '0 9 * * *',
    agent_id: 1,
    prompt: ''
  };

  async ngOnInit() {
    this.loadTasks();
    this.loadAgents();
    setInterval(() => this.loadTasks(), 15000);
  }

  async loadTasks() {
    const t = await this.api.getTasks();
    this.tasks.set(t);
  }

  async loadAgents() {
    this.agents.set(await this.api.getAgents());
  }

  toggleDay(key: string) {
    const idx = this.newTask.customDays.indexOf(key);
    if (idx >= 0) {
      this.newTask.customDays.splice(idx, 1);
    } else {
      this.newTask.customDays.push(key);
    }
    this.updateCronFromSelection();
  }

  updateCronFromSelection() {
    if (this.newTask.frequency === '@daily' || this.newTask.frequency === '@weekdays') {
      const [h, m] = this.newTask.scheduledHour.split(':');
      if (this.newTask.frequency === '@daily') {
        this.newTask.cron_expression = `${parseInt(m, 10)} ${parseInt(h, 10)} * * *`;
      } else {
        this.newTask.cron_expression = `${parseInt(m, 10)} ${parseInt(h, 10)} * * 1-5`;
      }
    } else if (this.newTask.frequency === 'custom') {
      const [h, m] = this.newTask.customHour.split(':');
      const minutes = parseInt(m, 10);
      const hours = parseInt(h, 10);
      const days = this.newTask.customDays.length > 0
        ? this.newTask.customDays.sort().join(',')
        : '*';
      this.newTask.cron_expression = `${minutes} ${hours} * * ${days}`;
    } else {
      this.newTask.cron_expression = this.newTask.frequency;
    }
  }

  onFrequencyChange() {
    this.updateCronFromSelection();
  }

  onHourChange() {
    this.updateCronFromSelection();
  }

  async createTask() {
    this.error.set('');

    if (!this.newTask.name.trim()) {
      this.error.set('El nombre es obligatorio.');
      return;
    }
    if (!this.newTask.prompt.trim()) {
      this.error.set('El prompt es obligatorio.');
      return;
    }

    this.updateCronFromSelection();

    this.isSaving.set(true);
    try {
      await this.api.createTask({
        name: this.newTask.name.trim(),
        cron_expression: this.newTask.cron_expression,
        agent_id: this.newTask.agent_id,
        prompt: this.newTask.prompt.trim(),
        one_shot: this.newTask.oneShot
      });
      this.newTask.name = '';
      this.newTask.prompt = '';
      this.newTask.customDays = [];
      this.newTask.oneShot = false;
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

  cronLabel(expr: string, oneShot: boolean): string {
    if (oneShot) return 'Ejecucion unica';
    switch (expr) {
      case '@hourly': return 'Cada hora';
      case '@daily': return 'Cada dia';
      case '* * * * *': return 'Cada minuto';
      default: return expr;
    }
  }
}
