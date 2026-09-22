import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';

export interface DistributionQueueRow {
  queue_name: string;
  received: number;
  answered: number;
  unanswered: number;
  sla_percent: number;
  avg_hold_seconds: number;
  avg_duration_seconds: number;
}

export interface DistributionSummary {
  sla_interval: number;
  short_abandons_excluded: number;
  range_clamped: boolean;
  queues: DistributionQueueRow[];
}

@Injectable({ providedIn: 'root' })
export class DistributionService {
  private readonly api = inject(ApiService);

  summary(params?: Record<string, unknown>): Observable<DistributionSummary> {
    return this.api.get<DistributionSummary>('secure/call-center/distribution', params);
  }
}
