import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';
import { Page } from '../../core/api/api.types';

/** Mirrors internal/scheduledreports.ListItem. Definitions only — see that
 * package's own doc comment: nothing executes these rows yet. */
export interface ScheduledReportRow {
  id: string;
  name: string;
  destination_email: string;
  reports: string[];
  queues: string[];
  last_days: number;
  cron_day_month: string;
  cron_day_week: string;
  cron_hour: string;
  cron_minute: string;
  is_active: boolean;
}

export interface ScheduledReportInput {
  name: string;
  destination_email: string;
  reports: string[];
  queues: string[];
  last_days: number;
  cron_day_month: string;
  cron_day_week: string;
  cron_hour: string;
  cron_minute: string;
  is_active: boolean;
}

/** The five report types heal-crm's own CreateModal offers — kept as the
 * same fixed vocabulary rather than free text, matching its
 * `in:distribution,answered,unanswered,agent,search` validation. */
export const REPORT_TYPES: { value: string; label: string }[] = [
  { value: 'distribution', label: 'Distribution' },
  { value: 'answered', label: 'Answered Calls' },
  { value: 'unanswered', label: 'Unanswered Calls' },
  { value: 'agent', label: 'Agent Performance' },
  { value: 'search', label: 'Call Search' },
];

@Injectable({ providedIn: 'root' })
export class ScheduledReportsService {
  private readonly api = inject(ApiService);

  list(params?: Record<string, unknown>): Observable<Page<ScheduledReportRow>> {
    return this.api.list<ScheduledReportRow>('secure/admin/scheduled-reports', params);
  }

  create(body: ScheduledReportInput): Observable<{ id: string }> {
    return this.api.post<{ id: string }>('secure/admin/scheduled-reports', body);
  }

  update(id: string, body: ScheduledReportInput): Observable<{ status: string }> {
    return this.api.put<{ status: string }>(`secure/admin/scheduled-reports/${id}`, body);
  }

  delete(id: string): Observable<{ status: string }> {
    return this.api.delete<{ status: string }>(`secure/admin/scheduled-reports/${id}`);
  }

  queueNames(): Observable<string[]> {
    return this.api.get<string[]>('secure/call-center/queues');
  }
}
