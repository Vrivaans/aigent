import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, AuditEvent, AuditEventsPage } from '../api.service';
import { TranslationService } from '../translation.service';

@Component({
  selector: 'app-audit',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './audit.html',
  styleUrl: './audit.css'
})
export class AuditComponent implements OnInit {
  private api = inject(ApiService);
  private translation = inject(TranslationService);

  t(key: string, params?: Record<string, string>): string {
    return this.translation.t(key, params);
  }

  events = signal<AuditEvent[]>([]);
  total = signal(0);
  limit = signal(50);
  offset = signal(0);
  isLoading = signal(false);

  filters = {
    from: '',
    to: '',
    actorUserId: '',
    action: '',
    resourceType: ''
  };

  async ngOnInit() {
    await this.loadEvents();
  }

  async loadEvents() {
    this.isLoading.set(true);
    try {
      const page = await this.fetchPage(this.offset());
      this.events.set(page.items);
      this.total.set(page.total);
      this.limit.set(page.limit);
      this.offset.set(page.offset);
    } catch (err) {
      console.error('Failed to load audit events:', err);
      alert(this.t('audit.error_load'));
    } finally {
      this.isLoading.set(false);
    }
  }

  private async fetchPage(offset: number): Promise<AuditEventsPage> {
    const params: Record<string, string | number> = {
      limit: this.limit(),
      offset
    };
    if (this.filters.from.trim()) params['from'] = this.filters.from.trim();
    if (this.filters.to.trim()) params['to'] = this.filters.to.trim();
    if (this.filters.actorUserId.trim()) params['actor_user_id'] = this.filters.actorUserId.trim();
    if (this.filters.action.trim()) params['action'] = this.filters.action.trim();
    if (this.filters.resourceType.trim()) params['resource_type'] = this.filters.resourceType.trim();
    return this.api.getAuditEvents(params);
  }

  async applyFilters() {
    this.offset.set(0);
    await this.loadEvents();
  }

  async clearFilters() {
    this.filters = { from: '', to: '', actorUserId: '', action: '', resourceType: '' };
    this.offset.set(0);
    await this.loadEvents();
  }

  async prevPage() {
    const next = Math.max(0, this.offset() - this.limit());
    this.offset.set(next);
    await this.loadEvents();
  }

  async nextPage() {
    if (this.offset() + this.limit() >= this.total()) return;
    this.offset.set(this.offset() + this.limit());
    await this.loadEvents();
  }

  pageLabel(): string {
    const start = this.total() === 0 ? 0 : this.offset() + 1;
    const end = Math.min(this.offset() + this.limit(), this.total());
    return this.t('audit.page_label', {
      start: '' + start,
      end: '' + end,
      total: '' + this.total()
    });
  }
}
