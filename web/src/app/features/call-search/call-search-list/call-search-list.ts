import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';

import { ApiMeta } from '../../../core/api/api.types';
import { IconComponent } from '../../../shared/components/icon/icon';
import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { formatDuration, today } from '../../../shared/date-range';
import {
  CallDetailRow,
  CallSearchResult,
  CallSearchService,
} from '../call-search.service';

@Component({
  selector: 'app-call-search-list',
  standalone: true,
  imports: [PageHeaderComponent, IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './call-search-list.html',
})
export class CallSearchListComponent {
  private readonly service = inject(CallSearchService);

  protected readonly rows = signal<CallSearchResult[]>([]);
  protected readonly meta = signal<ApiMeta | null>(null);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);

  protected readonly dateFrom = signal(today());
  protected readonly dateTo = signal(today());
  protected callerId = '';
  protected uniqueId = '';

  /** The call currently open in the detail drawer, or null. */
  protected readonly openCall = signal<CallSearchResult | null>(null);
  protected readonly timeline = signal<CallDetailRow[]>([]);
  protected readonly timelineLoading = signal(false);

  constructor() {
    this.search();
  }

  protected search(): void {
    this.loading.set(true);
    this.failed.set(false);

    const params: Record<string, unknown> = {
      date_from: `${this.dateFrom()} 00:00:00`,
      date_to: `${this.dateTo()} 23:59:59`,
    };
    if (this.callerId.trim()) {
      params['caller_id'] = this.callerId.trim();
    }
    if (this.uniqueId.trim()) {
      params['unique_id'] = this.uniqueId.trim();
    }

    this.service.search(params).subscribe({
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

  protected onDateFrom(event: Event): void {
    this.dateFrom.set((event.target as HTMLInputElement).value);
  }

  protected onDateTo(event: Event): void {
    this.dateTo.set((event.target as HTMLInputElement).value);
  }

  protected openDetail(row: CallSearchResult): void {
    this.openCall.set(row);
    this.timeline.set([]);
    this.timelineLoading.set(true);
    this.service.detail(row.uniqueid).subscribe({
      next: (rows) => {
        this.timeline.set(rows);
        this.timelineLoading.set(false);
      },
      error: () => {
        this.timelineLoading.set(false);
      },
    });
  }

  protected closeDetail(): void {
    this.openCall.set(null);
    this.timeline.set([]);
  }

  protected duration = formatDuration;
}
