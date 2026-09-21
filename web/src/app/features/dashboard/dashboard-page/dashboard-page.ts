import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';

import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { DashboardService, DashboardSummary } from '../dashboard.service';

@Component({
  selector: 'app-dashboard-page',
  standalone: true,
  imports: [PageHeaderComponent, DecimalPipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './dashboard-page.html',
})
export class DashboardPageComponent {
  private readonly service = inject(DashboardService);

  protected readonly summary = signal<DashboardSummary | null>(null);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);

  constructor() {
    this.load();
  }

  protected load(): void {
    this.loading.set(true);
    this.failed.set(false);

    this.service.summary().subscribe({
      next: (data) => {
        this.summary.set(data);
        this.loading.set(false);
      },
      error: () => {
        // The global dialog already said what went wrong; this only puts the
        // screen into a state that offers a retry rather than an empty page
        // that looks like "no calls today".
        this.failed.set(true);
        this.loading.set(false);
      },
    });
  }

  /** Seconds as m:ss, so a duration reads as a duration. */
  protected duration(seconds: number): string {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  }
}
