import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';

import { ApiMeta } from '../../../core/api/api.types';
import { IconComponent } from '../../../shared/components/icon/icon';
import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { formatDuration, today } from '../../../shared/date-range';
import { AnsweredCall, AnsweredCallsService } from '../answered-calls.service';

@Component({
  selector: 'app-answered-calls-list',
  standalone: true,
  imports: [PageHeaderComponent, IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './answered-calls-list.html',
})
export class AnsweredCallsListComponent {
  private readonly service = inject(AnsweredCallsService);

  protected readonly rows = signal<AnsweredCall[]>([]);
  protected readonly meta = signal<ApiMeta | null>(null);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);

  protected readonly searchTerm = signal('');
  protected readonly dateFrom = signal(today());
  protected readonly dateTo = signal(today());

  protected readonly visibleRows = computed(() => {
    const term = this.searchTerm().trim().toLowerCase();
    if (!term) {
      return this.rows();
    }
    return this.rows().filter((row) =>
      [row.caller_id, row.queue_name, row.agent_name].join(' ').toLowerCase().includes(term),
    );
  });

  /** Talk seconds summed across the loaded page, for a quick page total. */
  protected readonly totalTalkSeconds = computed(() =>
    this.rows().reduce((sum, row) => sum + row.duration, 0),
  );

  constructor() {
    this.load();
  }

  protected load(): void {
    this.loading.set(true);
    this.failed.set(false);
    this.service
      .list({ date_from: `${this.dateFrom()} 00:00:00`, date_to: `${this.dateTo()} 23:59:59` })
      .subscribe({
        next: (page) => {
          this.rows.set(page.rows);
          this.meta.set(page.meta);
          this.loading.set(false);
        },
        error: () => {
          this.failed.set(true);
          this.loading.set(false);
        },
      });
  }

  protected onSearchInput(event: Event): void {
    this.searchTerm.set((event.target as HTMLInputElement).value);
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
