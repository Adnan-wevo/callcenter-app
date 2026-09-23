import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';
import { Page } from '../../core/api/api.types';

/** Mirrors handlers.userFilterRow. Empty arrays mean unrestricted — see
 * auth.User's own doc comment; there is no separate "is restricted" flag. */
export interface UserFilterRow {
  id: string;
  username: string;
  allowed_queues: string[];
  allowed_agents: string[];
}

@Injectable({ providedIn: 'root' })
export class UserFiltersService {
  private readonly api = inject(ApiService);

  list(params?: Record<string, unknown>): Observable<Page<UserFilterRow>> {
    return this.api.list<UserFilterRow>('secure/admin/user-filters', params);
  }

  /** The picker options — the same queue/agent name lists the report
   * filter bars would use (pbx-worker's own queue-names/agent-names
   * actions), not this store's row-level restriction data. */
  queueNames(): Observable<string[]> {
    return this.api.get<string[]>('secure/call-center/queues');
  }

  agentNames(): Observable<string[]> {
    return this.api.get<string[]>('secure/call-center/agents');
  }

  update(userId: string, queues: string[], agents: string[]): Observable<{ status: string }> {
    return this.api.put<{ status: string }>(`secure/admin/user-filters/${userId}`, {
      queues,
      agents,
    });
  }

  clear(userId: string): Observable<{ status: string }> {
    return this.api.delete<{ status: string }>(`secure/admin/user-filters/${userId}`);
  }
}
