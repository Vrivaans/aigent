import { Component, OnInit, OnDestroy, signal, inject, ViewChild, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, Workflow, WorkflowRun } from '../api.service';
import { TranslationService } from '../translation.service';

@Component({
  selector: 'app-workflows',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './workflows.html',
  styleUrl: './workflows.css'
})
export class WorkflowsComponent implements OnInit, OnDestroy {
  private api = inject(ApiService);
  private translation = inject(TranslationService);

  t(key: string, params?: Record<string, string>): string {
    return this.translation.t(key, params);
  }

  workflows = signal<Workflow[]>([]);
  selectedWorkflow = signal<Workflow | null>(null);
  selectedWorkflowResponse = signal<any | null>(null);
  runs = signal<WorkflowRun[]>([]);
  selectedRun = signal<any | null>(null);
  activeView = signal<'preview' | 'json' | 'runs'>('preview');

  // Create new workflow form
  isCreating = signal(false);
  newName = '';
  newDescription = '';
  newCron = '';
  newDefinition = '{\n  "ruleChain": {\n    "name": "Nuevo Workflow",\n    "id": "new_wf"\n  },\n  "metadata": {\n    "nodes": [\n      {\n        "id": "n1",\n        "type": "aigent/tool",\n        "name": "Listar Odoo",\n        "configuration": {\n          "toolName": "odoo_sale_order_list"\n        }\n      }\n    ],\n    "connections": []\n  }\n}';

  // Run Execution Form
  runPayload = '{}';
  isExecuting = signal(false);

  // Loading states
  isLoadingList = signal(false);
  isLoadingDetail = signal(false);
  isLoadingRuns = signal(false);
  isLoadingRunDetail = signal(false);
  isReloading = signal(false);

  // Mermaid element references
  @ViewChild('designCanvas') designCanvas?: ElementRef;
  @ViewChild('runCanvas') runCanvas?: ElementRef;

  selectView(view: 'preview' | 'json' | 'runs') {
    this.activeView.set(view);
    if (view !== 'runs') {
      this.selectedRun.set(null);
    }
    setTimeout(() => this.renderCurrentGraph(), 50);
  }

  private pollInterval: any;

  async ngOnInit() {
    await this.loadWorkflows();
  }

  ngOnDestroy() {
    this.stopPolling();
  }

  async loadWorkflows() {
    this.isLoadingList.set(true);
    try {
      const list = await this.api.getWorkflows();
      this.workflows.set(list || []);
    } catch (err) {
      console.error('Failed to load workflows:', err);
    } finally {
      this.isLoadingList.set(false);
    }
  }

  async reloadEngine() {
    this.isReloading.set(true);
    try {
      await this.api.reloadWorkflows();
      await this.loadWorkflows();
    } catch (err: any) {
      console.error('Failed to reload workflow engine:', err);
      alert(this.t('workflows.reload_error', { msg: err.message || err }));
    } finally {
      this.isReloading.set(false);
    }
  }

  async selectWorkflow(wf: Workflow) {
    this.isCreating.set(false);
    this.selectedRun.set(null);
    this.stopPolling();
    this.isLoadingDetail.set(true);
    try {
      const details = await this.api.getWorkflow(wf.id);
      this.selectedWorkflow.set(wf);
      this.selectedWorkflowResponse.set(details);
      this.activeView.set('preview');
      setTimeout(() => this.renderCurrentGraph(), 50);
      await this.loadRuns(wf.id);
    } catch (err) {
      console.error('Failed to load workflow details:', err);
    } finally {
      this.isLoadingDetail.set(false);
    }
  }

  async loadRuns(workflowId: number) {
    this.isLoadingRuns.set(true);
    try {
      const list = await this.api.getWorkflowRuns(workflowId);
      this.runs.set(list || []);
    } catch (err) {
      console.error('Failed to load runs:', err);
    } finally {
      this.isLoadingRuns.set(false);
    }
  }

  async selectRun(run: WorkflowRun) {
    this.stopPolling();
    this.isLoadingRunDetail.set(true);
    try {
      const runDetails = await this.api.getWorkflowRun(run.id);
      this.selectedRun.set(runDetails);
      setTimeout(() => this.renderCurrentGraph(), 50);

      if (runDetails.status === 'RUNNING') {
        this.startPolling(run.id);
      }
    } catch (err) {
      console.error('Failed to load run details:', err);
    } finally {
      this.isLoadingRunDetail.set(false);
    }
  }

  startPolling(runId: number) {
    this.pollInterval = setInterval(async () => {
      try {
        const runDetails = await this.api.getWorkflowRun(runId);
        this.selectedRun.set(runDetails);
        this.renderCurrentGraph();

        // Refresh runs list
        const wf = this.selectedWorkflow();
        if (wf) {
          const list = await this.api.getWorkflowRuns(wf.id);
          this.runs.set(list || []);
        }

        if (runDetails.status !== 'RUNNING') {
          this.stopPolling();
        }
      } catch (err) {
        console.error('Error polling run details:', err);
        this.stopPolling();
      }
    }, 2000);
  }

  stopPolling() {
    if (this.pollInterval) {
      clearInterval(this.pollInterval);
      this.pollInterval = null;
    }
  }

  async renderCurrentGraph() {
    let container: HTMLElement | null = null;
    let code = '';

    if (this.selectedRun()) {
      if (!this.runCanvas) return;
      container = this.runCanvas.nativeElement;
      code = this.selectedRun().mermaid;
    } else if (this.selectedWorkflowResponse() && this.activeView() === 'preview') {
      if (!this.designCanvas) return;
      container = this.designCanvas.nativeElement;
      code = this.selectedWorkflowResponse().mermaid;
    }

    if (!container) return;

    if (!code) {
      container.innerHTML = `<div class="empty-mermaid">${this.t('workflows.no_graph')}</div>`;
      return;
    }

    try {
      const id = `mermaid-workflow-svg-${Date.now()}`;
      const mermaid = (await import('mermaid')).default;
      mermaid.initialize({
        startOnLoad: false,
        theme: 'dark',
        securityLevel: 'loose',
        flowchart: {
          useMaxWidth: false,
          htmlLabels: true
        }
      });
      const { svg } = await mermaid.render(id, code);
      container.innerHTML = `<div class="mermaid-svg-wrapper">${svg}</div>`;
    } catch (err: any) {
      console.error('Failed to render mermaid diagram:', err);
      container.innerHTML = `<div class="mermaid-error">${this.t('workflows.render_error', { msg: err.message || err })}</div>`;
    }
  }

  async executeWorkflow() {
    const wf = this.selectedWorkflow();
    if (!wf) return;
    this.isExecuting.set(true);
    try {
      const res = await this.api.runWorkflow(wf.id, this.runPayload);
      await this.loadRuns(wf.id);
      if (res.run_id) {
        const newRun = this.runs().find(r => r.id === res.run_id);
        if (newRun) {
          await this.selectRun(newRun);
        } else {
          const list = await this.api.getWorkflowRuns(wf.id);
          this.runs.set(list || []);
          if (this.runs().length > 0) {
            await this.selectRun(this.runs()[0]);
          }
        }
      }
    } catch (err) {
      console.error('Failed to run workflow:', err);
      alert(this.t('workflows.run_error'));
    } finally {
      this.isExecuting.set(false);
    }
  }

  async deleteWorkflow(wf: Workflow, event: Event) {
    event.stopPropagation();
    if (!confirm(this.t('workflows.delete_confirm', { name: wf.name }))) return;
    try {
      await this.api.deleteWorkflow(wf.id);
      if (this.selectedWorkflow()?.id === wf.id) {
        this.selectedWorkflow.set(null);
        this.selectedWorkflowResponse.set(null);
        this.selectedRun.set(null);
        this.stopPolling();
      }
      await this.loadWorkflows();
    } catch (err) {
      console.error('Failed to delete workflow:', err);
      alert(this.t('workflows.delete_error'));
    }
  }

  startCreate() {
    this.selectedWorkflow.set(null);
    this.selectedWorkflowResponse.set(null);
    this.selectedRun.set(null);
    this.stopPolling();
    this.isCreating.set(true);
    this.newName = '';
    this.newDescription = '';
    this.newCron = '';
    this.newDefinition = '{\n  "ruleChain": {\n    "name": "Nuevo Workflow",\n    "id": "new_wf"\n  },\n  "metadata": {\n    "nodes": [\n      {\n        "id": "n1",\n        "type": "aigent/tool",\n        "name": "Obtener Órdenes",\n        "configuration": {\n          "toolName": "odoo_sale_order_list"\n        }\n      }\n    ],\n    "connections": []\n  }\n}';
  }

  async saveWorkflow() {
    if (!this.newName.trim()) {
      alert(this.t('workflows.name_required'));
      return;
    }
    try {
      JSON.parse(this.newDefinition);
    } catch (e: any) {
      alert(this.t('workflows.invalid_json', { msg: e.message }));
      return;
    }

    try {
      const created = await this.api.createWorkflow({
        name: this.newName,
        description: this.newDescription,
        cron_expression: this.newCron || undefined,
        definition: this.newDefinition
      });
      this.isCreating.set(false);
      await this.loadWorkflows();
      await this.selectWorkflow(created);
    } catch (err: any) {
      console.error('Failed to create workflow:', err);
      alert(this.t('workflows.create_error', { msg: err.message || err }));
    }
  }

  formatJson(jsonStr: string): string {
    try {
      return JSON.stringify(JSON.parse(jsonStr), null, 2);
    } catch {
      return jsonStr;
    }
  }
}
