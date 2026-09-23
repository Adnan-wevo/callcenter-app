import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';
import { Page } from '../../core/api/api.types';

/** Mirrors internal/queuegroups.ListItem. */
export interface QueueGroupRow {
  id: string;
  name: string;
  description: string;
  is_active: boolean;
  sort_order: number;
  queues: string[];
}

export interface QueueGroupInput {
  name: string;
  description: string;
  is_active: boolean;
  sort_order: number;
  queues: string[];
}

@Injectable({ providedIn: 'root' })
export class QueueGroupsService {
  private readonly api = inject(ApiService);

  list(params?: Record<string, unknown>): Observable<Page<QueueGroupRow>> {
    return this.api.list<QueueGroupRow>('secure/admin/queue-groups', params);
  }

  create(body: QueueGroupInput): Observable<{ id: string }> {
    return this.api.post<{ id: string }>('secure/admin/queue-groups', body);
  }

  update(id: string, body: QueueGroupInput): Observable<{ status: string }> {
    return this.api.put<{ status: string }>(`secure/admin/queue-groups/${id}`, body);
  }

  delete(id: string): Observable<{ status: string }> {
    return this.api.delete<{ status: string }>(`secure/admin/queue-groups/${id}`);
  }

  /** The picker options — same source SIP Extensions/User Filters/Realtime
   * Monitor already use. */
  queueNames(): Observable<string[]> {
    return this.api.get<string[]>('secure/call-center/queues');
  }
}
