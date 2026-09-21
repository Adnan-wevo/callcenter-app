import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';
import { Page } from '../../core/api/api.types';

/**
 * One unanswered-call row, as pbx-worker's `unanswered-calls` action returns
 * it. `event` is one of the four real CallEvent cases — ABANDON,
 * EXITWITHTIMEOUT, EXITWITHKEY, EXITEMPTY — not a free-text reason.
 */
export interface UnansweredCall {
  datetime: string;
  queue_name: string;
  agent_name: string;
  event: string;
  uniqueid: string;
  caller_id: string;
  url: string;
  did: string;
  ring_time: number;
  hold_time: number;
  year_month: string;
  year_week: string;
  date: string;
  hour: number;
  day_of_week: number;
}

export interface CallbackResult {
  phone_number: string;
  agent_id: string;
  outcome: string;
}

@Injectable({ providedIn: 'root' })
export class UnansweredCallsService {
  private readonly api = inject(ApiService);

  list(params?: Record<string, unknown>): Observable<Page<UnansweredCall>> {
    return this.api.list<UnansweredCall>('secure/call-center/unanswered-calls', params);
  }

  /**
   * Record a callback attempt.
   *
   * Only the phone number is sent: the acting agent is taken from the bearer
   * token on the server, never from here. That mirrors the Livewire original,
   * which passes `Auth::id()` and never a value from the page.
   */
  callback(phoneNumber: string): Observable<CallbackResult> {
    return this.api.post<CallbackResult>('secure/call-center/unanswered-calls/callback', {
      phone_number: phoneNumber,
    });
  }
}
