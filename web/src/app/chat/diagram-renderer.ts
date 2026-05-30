import { Component, Input, Output, EventEmitter, ElementRef, ViewChild, OnChanges, SimpleChanges, AfterViewInit, signal } from '@angular/core';

@Component({
  selector: 'app-diagram-renderer',
  standalone: true,
  template: `
    <div class="diagram-wrapper">
      <div class="zoom-controls">
        <button (click)="zoomIn()" title="Acercar">➕</button>
        <button (click)="zoomOut()" title="Alejar">➖</button>
        <button (click)="resetZoom()" title="Restablecer">🔄</button>
      </div>

      <div #mermaidContainer class="mermaid-container"
           (mousedown)="onMouseDown($event)"
           (mousemove)="onMouseMove($event)"
           (mouseup)="onMouseUp()"
           (mouseleave)="onMouseLeave()"
           (wheel)="onWheel($event)">
      </div>

      @if (selectedNode()) {
        <div class="node-details-card">
          <div class="card-header">
            <span class="card-icon">📌</span>
            <span class="card-title">{{ selectedNode() }}</span>
            <button class="card-close" (click)="selectedNode.set(null)">✕</button>
          </div>
          <div class="card-body">
            <p>Has seleccionado este nodo. Puedes realizar una consulta interactiva al agente o cerrar este panel.</p>
          </div>
          <div class="card-footer">
            <button class="btn-ask-agent" (click)="askAboutNode(selectedNode()!)">
              💬 Consultar al Agente
            </button>
          </div>
        </div>
      }
    </div>
  `,
  styles: [`
    .diagram-wrapper {
      position: relative;
      width: 100%;
      height: 100%;
      overflow: hidden;
      display: flex;
      justify-content: center;
      align-items: center;
      background-color: #1e1e2e;
      padding: 0;
      border-radius: 8px;
    }
    
    .mermaid-container {
      width: 100%;
      height: 100%;
      cursor: grab;
      display: flex;
      justify-content: center;
      align-items: center;
      user-select: none;
    }
    
    .mermaid-container:active {
      cursor: grabbing;
    }
    
    ::ng-deep .mermaid-svg-wrapper {
      display: inline-block;
      transform-origin: center;
      transition: transform 0.05s ease-out;
    }
    
    ::ng-deep .mermaid-container svg {
      max-width: 100%;
      height: auto;
    }

    /* Zoom Controls styling */
    .zoom-controls {
      position: absolute;
      top: 15px;
      right: 15px;
      display: flex;
      flex-direction: column;
      gap: 6px;
      z-index: 10;
      background: rgba(17, 17, 27, 0.85);
      backdrop-filter: blur(8px);
      padding: 5px;
      border-radius: 8px;
      border: 1px solid rgba(255, 255, 255, 0.05);
    }

    .zoom-controls button {
      background: transparent;
      border: none;
      color: #cdd6f4;
      cursor: pointer;
      width: 32px;
      height: 32px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 14px;
      border-radius: 6px;
      transition: background-color 0.2s;
    }

    .zoom-controls button:hover {
      background-color: rgba(255, 255, 255, 0.1);
    }

    /* Node Details Card styling */
    .node-details-card {
      position: absolute;
      bottom: 20px;
      left: 50%;
      transform: translateX(-50%);
      width: calc(100% - 40px);
      max-width: 360px;
      background: rgba(24, 24, 37, 0.95);
      backdrop-filter: blur(12px);
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 12px;
      box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
      z-index: 10;
      padding: 16px;
      animation: slideUp 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
    }

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 8px;
    }

    .card-icon {
      font-size: 16px;
      margin-right: 6px;
    }

    .card-title {
      font-size: 14px;
      font-weight: 600;
      color: #cdd6f4;
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .card-close {
      background: transparent;
      border: none;
      color: #a6adc8;
      cursor: pointer;
      font-size: 14px;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 20px;
      height: 20px;
      border-radius: 50%;
      transition: background-color 0.2s;
    }

    .card-close:hover {
      background-color: rgba(255, 255, 255, 0.1);
      color: #f38ba8;
    }

    .card-body {
      font-size: 12px;
      color: #bac2de;
      line-height: 1.5;
      margin-bottom: 12px;
    }

    .card-footer {
      display: flex;
      justify-content: flex-end;
    }

    .btn-ask-agent {
      background: linear-gradient(135deg, #89b4fa, #b4befe);
      color: #11111b;
      border: none;
      padding: 6px 12px;
      font-size: 12px;
      font-weight: 600;
      border-radius: 6px;
      cursor: pointer;
      transition: transform 0.2s, box-shadow 0.2s;
    }

    .btn-ask-agent:hover {
      transform: translateY(-1px);
      box-shadow: 0 4px 12px rgba(137, 180, 250, 0.3);
    }

    @keyframes slideUp {
      from {
        transform: translate(-50%, 20px);
        opacity: 0;
      }
      to {
        transform: translate(-50%, 0);
        opacity: 1;
      }
    }
  `]
})
export class DiagramRenderer implements OnChanges, AfterViewInit {
  @Input({ required: true }) code!: string;
  @Input() title: string = '';
  @Output() nodeClicked = new EventEmitter<string>();

  @ViewChild('mermaidContainer') mermaidContainer!: ElementRef;

  // Zoom and Pan states
  scale = 1;
  translateX = 0;
  translateY = 0;
  isDragging = false;
  startX = 0;
  startY = 0;

  selectedNode = signal<string | null>(null);
  private isViewInit = false;

  ngAfterViewInit() {
    this.isViewInit = true;
    this.renderDiagram();
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['code'] && this.isViewInit) {
      this.renderDiagram();
    }
  }

  zoomIn() {
    this.scale = Math.min(this.scale * 1.2, 5);
    this.applyTransform();
  }

  zoomOut() {
    this.scale = Math.max(this.scale / 1.2, 0.2);
    this.applyTransform();
  }

  resetZoom() {
    this.scale = 1;
    this.translateX = 0;
    this.translateY = 0;
    this.applyTransform();
  }

  onWheel(event: WheelEvent) {
    event.preventDefault();
    const zoomFactor = 1.1;
    if (event.deltaY < 0) {
      this.scale = Math.min(this.scale * zoomFactor, 5);
    } else {
      this.scale = Math.max(this.scale / zoomFactor, 0.2);
    }
    this.applyTransform();
  }

  onMouseDown(event: MouseEvent) {
    if (event.button === 0) {
      const target = event.target as HTMLElement;
      if (target.closest('.node-details-card') || target.closest('.zoom-controls')) {
        return;
      }
      this.isDragging = true;
      this.startX = event.clientX - this.translateX;
      this.startY = event.clientY - this.translateY;
    }
  }

  onMouseMove(event: MouseEvent) {
    if (this.isDragging) {
      this.translateX = event.clientX - this.startX;
      this.translateY = event.clientY - this.startY;
      this.applyTransform();
    }
  }

  onMouseUp() {
    this.isDragging = false;
  }

  onMouseLeave() {
    this.isDragging = false;
  }

  askAboutNode(nodeLabel: string) {
    this.nodeClicked.emit(nodeLabel);
    this.selectedNode.set(null);
  }

  private async renderDiagram() {
    if (!this.code || !this.mermaidContainer) return;
    try {
      const container = this.mermaidContainer.nativeElement;
      container.innerHTML = '';
      
      const id = `mermaid-svg-${Date.now()}`;
      const mermaid = (await import('mermaid')).default;
      
      mermaid.initialize({
        startOnLoad: false,
        theme: 'dark',
        securityLevel: 'loose',
        themeVariables: {
          background: '#1e1e2e',
          primaryColor: '#89b4fa',
          primaryTextColor: '#cdd6f4',
          lineColor: '#a6adc8',
          arrowheadColor: '#f38ba8'
        }
      });

      const { svg } = await mermaid.render(id, this.code);
      container.innerHTML = `<div class="mermaid-svg-wrapper">${svg}</div>`;

      // Reset transforms for new diagram
      this.scale = 1;
      this.translateX = 0;
      this.translateY = 0;
      this.applyTransform();

      this.bindNodeEvents();
    } catch (error) {
      console.error('Error rendering diagram:', error);
      this.mermaidContainer.nativeElement.innerHTML = `
        <div style="color: #f38ba8; padding: 10px; border: 1px solid #f38ba8; border-radius: 4px; background-color: #313244; font-family: monospace;">
          <strong>Error al renderizar el diagrama:</strong><br>
          <pre style="white-space: pre-wrap; font-size: 12px; margin-top: 5px;">${error}</pre>
        </div>
      `;
    }
  }

  private applyTransform() {
    const wrapper = this.mermaidContainer.nativeElement.querySelector('.mermaid-svg-wrapper');
    if (wrapper) {
      wrapper.style.transform = `translate(${this.translateX}px, ${this.translateY}px) scale(${this.scale})`;
    }
  }

  private bindNodeEvents() {
    const wrapper = this.mermaidContainer.nativeElement.querySelector('.mermaid-svg-wrapper');
    if (!wrapper) return;

    const nodes = wrapper.querySelectorAll('.node, .cluster');
    nodes.forEach((node: Element) => {
      const labelEl = node.querySelector('.label') || node.querySelector('text');
      const nodeText = labelEl ? labelEl.textContent?.trim() : '';
      
      if (nodeText) {
        (node as HTMLElement).style.cursor = 'pointer';
        
        // Left click: open node details drawer
        node.addEventListener('click', (e) => {
          e.preventDefault();
          e.stopPropagation();
          this.selectedNode.set(nodeText);
        });

        // Right click: query agent directly
        node.addEventListener('contextmenu', (e) => {
          e.preventDefault();
          e.stopPropagation();
          this.nodeClicked.emit(nodeText);
        });
      }
    });
  }
}
