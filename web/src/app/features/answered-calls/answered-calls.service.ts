import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';
import { Page } from '../../core/api/api.types';

/** One answered-call row, as pbx-worker's `answered-calls` action returns it. */
export interface AnsweredCall {
  datetime: string;
  queue_name: string;
  agent_name: string;
  event: string;
  uniqueid: string;
  caller_id: string;
  url: string;
  did: string;
  ring_time: number;
  recording_file: string;
  hold_time: number;
  duration: number;
  position: number;
  transfer_exten: string;
  year_month: string;
  year_week: string;
  date: string;
  hour: number;
  day_of_week: number;
  seconds_of_day: number;
}

@Injectable({ providedIn: 'root' })
export class AnsweredCallsService {
  private readonly api = inject(ApiService);

  list(params?: Record<string, unknown>): Observable<Page<AnsweredCall>> {
    return this.api.list<AnsweredCall>('secure/call-center/answered-calls', params);
  }
}
