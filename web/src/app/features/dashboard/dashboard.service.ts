import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';

/** One queue's line in the distribution block. */
export interface QueueRow {
  queue_name: string;
  answered: number;
  unanswered: number;
  received: number;
}

export interface DashboardSummary {
  answered: number;
  unanswered: number;
  received: number;
  avg_hold_seconds: number;
  avg_duration_seconds: number;
  sla_percent: number;
  sla_interval: number;
  /**
   * How many unanswered calls the short-abandon threshold suppressed. The
   * server drops them from every figure, as Laravel does; surfacing the
   * count keeps the rule visible instead of leaving a number that looks
   * wrong next to a raw switch report.
   */
  short_abandons_excluded: number;
  /** True when the requested window exceeded 90 days and was narrowed. */
  range_clamped: boolean;
  distribution: QueueRow[];
}

@Injectable({ providedIn: 'root' })
export class DashboardService {
  private readonly api = inject(ApiService);

  summary(params?: Record<string, unknown>): Observable<DashboardSummary> {
    return this.api.get<DashboardSummary>('secure/call-center/dashboard/summary', params);
  }
}
