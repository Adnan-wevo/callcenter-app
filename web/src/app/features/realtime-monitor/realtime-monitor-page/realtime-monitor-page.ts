import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject, signal } from '@angular/core';

import { HasPermissionDirective } from '../../../core/authz/has-permission.directive';
import { LiveAgentsSnapshot, LiveQueuesSnapshot } from '../../../core/softphone/softphone.types';
import { IconComponent } from '../../../shared/components/icon/icon';
import { PageHeaderComponent } from '../../../shared/components/page-header/page-header';
import { NotificationService } from '../../../shared/services/notification.service';
import { RealtimeMonitorService } from '../realtime-monitor.service';

interface QueueSummaryRow {
  id: string;
  waiting: number;
  staffed: number;
  paused: number;
  available: number;
  completedToday: number;
  abandonedToday: number;
}

interface WaitingCallRow {
  queue: string;
  callerId: string;
  position: number;
  waitSeconds: number;
  channel: string;
  uniqueId: string;
}

const POLL_MS = 8000;

/**
 * The live supervisor board — queue summary, agent statuses, waiting calls
 * — with pause/unpause/logout/redirect actions over ANY agent, not just
 * the viewer's own (see `RealtimeMonitorService`'s own doc comment on why
 * those are separate endpoints from the softphone panel's self-service
 * ones). Mirrors `Modules/CallCenter/app/Livewire/RealtimeMonitor/Index.php`
 * minus its Echo/Reverb push layer — this polls, same simplification the
 * softphone panel's own live snapshots already made.
 */
@Component({
  selector: 'app-realtime-monitor-page',
  standalone: true,
  imports: [IconComponent, PageHeaderComponent, HasPermissionDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './realtime-monitor-page.html',
})
export class RealtimeMonitorPageComponent {
  private readonly service = inject(RealtimeMonitorService);
  private readonly notify = inject(NotificationService);

  protected readonly queues = signal<LiveQueuesSnapshot | null>(null);
  protected readonly agentsSnapshot = signal<LiveAgentsSnapshot | null>(null);
  protected readonly loading = signal(true);
  protected readonly failed = signal(false);
  protected readonly autoRefresh = signal(true);

  protected readonly redirectingChannel = signal<string | null>(null);
  protected readonly redirectTarget = signal('');

  protected readonly summary = computed<QueueSummaryRow[]>(() => {
    const snap = this.queues();
    if (!snap) {
      return [];
    }
    return Object.entries(snap.queues).map(([id, q]) => {
      const paused = q.members.filter((m) => m.paused).length;
      return {
        id: q.name || id,
        waiting: q.calls_waiting,
        staffed: q.members.length,
        paused,
        available: Math.max(0, q.members.length - paused),
        completedToday: q.completed_today,
        abandonedToday: q.abandoned_today,
      };
    });
  });

  protected readonly totals = computed(() => {
    const rows = this.summary();
    return rows.reduce(
      (acc, r) => ({
        waiting: acc.waiting + r.waiting,
        staffed: acc.staffed + r.staffed,
        paused: acc.paused + r.paused,
        available: acc.available + r.available,
        completedToday: acc.completedToday + r.completedToday,
        abandonedToday: acc.abandonedToday + r.abandonedToday,
      }),
      { waiting: 0, staffed: 0, paused: 0, available: 0, completedToday: 0, abandonedToday: 0 },
    );
  });

  protected readonly waitingCalls = computed<WaitingCallRow[]>(() => {
    const snap = this.queues();
    if (!snap) {
      return [];
    }
    const rows: WaitingCallRow[] = [];
    for (const [id, q] of Object.entries(snap.queues)) {
      for (const caller of q.callers) {
        rows.push({
          queue: q.name || id,
          callerId: caller.caller_id,
          position: caller.position,
          waitSeconds: caller.wait_seconds,
          channel: caller.channel,
          uniqueId: caller.unique_id,
        });
      }
    }
    return rows.sort((a, b) => b.waitSeconds - a.waitSeconds);
  });

  protected readonly agentRows = computed(() => {
    const snap = this.agentsSnapshot();
    return snap ? Object.values(snap.agents) : [];
  });

  private pollTimer: ReturnType<typeof setInterval> | null = null;

  constructor() {
    this.refresh();
    this.startPolling();
    inject(DestroyRef).onDestroy(() => this.stopPolling());
  }

  private startPolling(): void {
    this.stopPolling();
    this.pollTimer = setInterval(() => {
      if (this.autoRefresh()) {
        this.refresh();
      }
    }, POLL_MS);
  }

  private stopPolling(): void {
    if (this.pollTimer) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
  }

  protected refresh(): void {
    this.loading.set(this.queues() === null);
    this.failed.set(false);
    this.service.queues().subscribe({
      next: (envelope) => {
        this.queues.set(envelope.data);
        this.loading.set(false);
      },
      error: () => {
        this.loading.set(false);
        this.failed.set(true);
      },
    });
    this.service.agents().subscribe({ next: (envelope) => this.agentsSnapshot.set(envelope.data) });
  }

  protected wait(seconds: number): string {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  }

  protected pause(iface: string): void {
    this.service.pauseAgent(iface, '').subscribe({
      next: () => {
        this.notify.success(`${iface} paused.`);
        this.refresh();
      },
      error: () => this.notify.error(`Could not pause ${iface}.`),
    });
  }

  protected unpause(iface: string): void {
    this.service.unpauseAgent(iface, '').subscribe({
      next: () => {
        this.notify.success(`${iface} resumed.`);
        this.refresh();
      },
      error: () => this.notify.error(`Could not resume ${iface}.`),
    });
  }

  protected logout(iface: string): void {
    if (!confirm(`Log ${iface} out of every queue?`)) {
      return;
    }
    this.service.logoutAgent(iface).subscribe({
      next: () => {
        this.notify.success(`${iface} logged out.`);
        this.refresh();
      },
      error: () => this.notify.error(`Could not log out ${iface}.`),
    });
  }

  protected startRedirect(channel: string): void {
    this.redirectingChannel.set(channel);
    this.redirectTarget.set('');
  }

  protected sendRedirect(channel: string): void {
    const target = this.redirectTarget().trim();
    if (!target) {
      this.redirectingChannel.set(null);
      return;
    }
    this.service.redirectCall(channel, target).subscribe({
      next: () => {
        this.notify.success(`Redirected to ${target}.`);
        this.refresh();
      },
      error: () => this.notify.error('Could not redirect that call.'),
    });
    this.redirectingChannel.set(null);
  }
}
