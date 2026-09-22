import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';

import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { formatDuration, today } from '../../../shared/date-range';
import { AgentPerformanceService, AgentStats } from '../agent-performance.service';

@Component({
  selector: 'app-agent-performance-page',
  standalone: true,
  imports: [PageHeaderComponent, DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './agent-performance-page.html',
})
export class AgentPerformancePageComponent {
  private readonly service = inject(AgentPerformanceService);

  protected readonly agents = signal<AgentStats[]>([]);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);

  protected readonly dateFrom = signal(today());
  protected readonly dateTo = signal(today());

  constructor() {
    this.load();
  }

  protected load(): void {
    this.loading.set(true);
    this.failed.set(false);
    this.service
      .summary({ date_from: `${this.dateFrom()} 00:00:00`, date_to: `${this.dateTo()} 23:59:59` })
      .subscribe({
        next: (data) => {
          this.agents.set(data.agents);
          this.loading.set(false);
        },
        error: () => {
          this.failed.set(true);
          this.loading.set(false);
        },
      });
  }

  protected onDateFrom(event: Event): void {
    this.dateFrom.set((event.target as HTMLInputElement).value);
    this.load();
  }

  protected onDateTo(event: Event): void {
    this.dateTo.set((event.target as HTMLInputElement).value);
    this.load();
  }

  protected duration = formatDuration;
}
