import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';
import {
  LiveAgentsSnapshot,
  LiveQueuesSnapshot,
  LiveSnapshotEnvelope,
} from '../../core/softphone/softphone.types';

/**
 * The live board — same live snapshot endpoints `SoftphoneService` polls
 * for the softphone panel (`/secure/softphone/queues`/`/agents`, see that
 * service's own doc comment on the double-`data` envelope shape), fetched
 * independently here rather than sharing that service's polling loop: this
 * page's lifecycle (mount/unmount as a report screen) is unrelated to
 * whether the softphone itself is registered.
 */
@Injectable({ providedIn: 'root' })
export class RealtimeMonitorService {
  private readonly api = inject(ApiService);

  queues(): Observable<LiveSnapshotEnvelope<LiveQueuesSnapshot>> {
    return this.api.get<LiveSnapshotEnvelope<LiveQueuesSnapshot>>('secure/softphone/queues', undefined, {
      silent: true,
    });
  }

  agents(): Observable<LiveSnapshotEnvelope<LiveAgentsSnapshot>> {
    return this.api.get<LiveSnapshotEnvelope<LiveAgentsSnapshot>>('secure/softphone/agents', undefined, {
      silent: true,
    });
  }

  // --- Supervisor actions — over an arbitrary agent, gated server-side by
  // softphone.supervise (router.go's `supervise` group). Distinct from
  // SoftphoneService's self-service queuePause/queueLogout, which are
  // permanently locked to the CALLER's own interface.

  pauseAgent(iface: string, queue: string, reason = ''): Observable<unknown> {
    return this.api.post('secure/softphone/queue/agent-pause', { interface: iface, queue, reason });
  }

  unpauseAgent(iface: string, queue: string): Observable<unknown> {
    return this.api.post('secure/softphone/queue/agent-unpause', { interface: iface, queue });
  }

  logoutAgent(iface: string, queues: string[] = ['all']): Observable<unknown> {
    return this.api.post('secure/softphone/queue/agent-logout', { interface: iface, queues });
  }

  redirectCall(channel: string, extension: string): Observable<unknown> {
    return this.api.post('secure/softphone/queue/redirect', { channel, extension });
  }
}
