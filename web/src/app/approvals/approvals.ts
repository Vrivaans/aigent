import { Component, OnInit, signal, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ApiService, ApprovalHistoryItem, PendingApproval } from '../api.service';
import { TranslationService } from '../translation.service';

@Component({
  selector: 'app-approvals',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './approvals.html',
  styleUrl: './approvals.css'
})
export class ApprovalsComponent implements OnInit {
  private api = inject(ApiService);
  private translation = inject(TranslationService);

  t(key: string, params?: Record<string, string>): string {
    return this.translation.t(key, params);
  }

  approvals = signal<PendingApproval[]>([]);
  history = signal<ApprovalHistoryItem[]>([]);
  isLoading = signal(false);

  async ngOnInit() {
    await this.loadApprovals();
  }

  async loadApprovals() {
    this.isLoading.set(true);
    try {
      const [pending, resolved] = await Promise.all([
        this.api.getPendingApprovals(),
        this.api.getApprovalHistory()
      ]);
      this.approvals.set(pending || []);
      this.history.set(resolved || []);
    } catch (err) {
      console.error('Failed to load pending approvals:', err);
    } finally {
      this.isLoading.set(false);
    }
  }

  formatJson(jsonStr: string): string {
    try {
      const parsed = JSON.parse(jsonStr);
      return JSON.stringify(parsed, null, 2);
    } catch (e) {
      return jsonStr;
    }
  }

  statusLabel(status: string): string {
    if (status === 'APPROVED') return this.t('approvals.status_approved');
    if (status === 'REJECTED') return this.t('approvals.status_rejected');
    return status;
  }

  async approve(app: PendingApproval) {
    try {
      await this.api.confirmAction(app.session_id, app.id, true);
      await this.loadApprovals();
    } catch (err) {
      console.error('Failed to approve action:', err);
      alert(this.t('approvals.error_approve'));
    }
  }

  async reject(app: PendingApproval) {
    try {
      await this.api.confirmAction(app.session_id, app.id, false);
      await this.loadApprovals();
    } catch (err) {
      console.error('Failed to reject action:', err);
      alert(this.t('approvals.error_reject'));
    }
  }
}
