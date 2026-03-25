import { Component, signal, inject, OnInit } from '@angular/core';
import { ApiService, Task } from '../api.service';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.css'
})
export class Dashboard implements OnInit {
  private api = inject(ApiService);
  tasks = signal<Task[]>([]);

  async ngOnInit() {
    this.loadTasks();
    setInterval(() => this.loadTasks(), 15000); // Live reload MVP
  }

  async loadTasks() {
    const t = await this.api.getTasks();
    this.tasks.set(t);
  }

  async deleteTask(id: number) {
    await this.api.deleteTask(id);
    this.loadTasks();
  }

  formatDate(d: string): string {
    return new Date(d).toLocaleString();
  }
}
