import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';

export interface AgentStats {
  agent_name: string;
  calls_handled: number;
  avg_talk_seconds: number;
  avg_hold_seconds: number;
  session_seconds: number;
  pause_seconds: number;
  wrap_up_seconds: number;
  occupancy_percent: number;
}

export interface AgentPerformanceResponse {
  agents: AgentStats[];
  range_clamped: boolean;
}

@Injectable({ providedIn: 'root' })
export class AgentPerformanceService {
  private readonly api = inject(ApiService);

  summary(params?: Record<string, unknown>): Observable<AgentPerformanceResponse> {
    return this.api.get<AgentPerformanceResponse>('secure/call-center/agent-performance', params);
  }
}
