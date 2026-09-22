import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';
import { Page } from '../../core/api/api.types';

export interface CallSearchResult {
  uniqueid: string;
  caller_id: string;
  date_start: string;
  date_end: string;
  event: string;
  agent_name: string;
  queue_name: string;
  talk_time: number;
  total_duration: number;
  wait_time: number;
  queue_hops: number;
  recording_file: string;
}

/** One timeline row for a single call — the call-detail action's shape. */
export interface CallDetailRow {
  datetime: string;
  queue_name: string;
  agent_name: string;
  event: string;
  info1: string;
  info2: string;
  info3: string;
  uniqueid: string;
  recording_file: string;
}

@Injectable({ providedIn: 'root' })
export class CallSearchService {
  private readonly api = inject(ApiService);

  search(params?: Record<string, unknown>): Observable<Page<CallSearchResult>> {
    return this.api.list<CallSearchResult>('secure/call-center/calls/search', params);
  }

  /** The response is an ARRAY of timeline rows, not a single object. */
  detail(uniqueId: string): Observable<CallDetailRow[]> {
    return this.api.get<CallDetailRow[]>(`secure/call-center/calls/${encodeURIComponent(uniqueId)}`);
  }
}
