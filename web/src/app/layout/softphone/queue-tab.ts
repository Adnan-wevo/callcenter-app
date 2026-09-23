import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from '@angular/core';

import { SoftphoneService } from '../../core/softphone/softphone.service';
import { IconComponent } from '../../shared/components/icon/icon';
import { PagerComponent } from '../../shared/components/pager/pager';

@Component({
  selector: 'app-queue-tab',
  standalone: true,
  imports: [IconComponent, PagerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './queue-tab.html',
})
export class QueueTabComponent {
  protected readonly phone = inject(SoftphoneService);

  protected readonly queueInput = signal('');
  protected readonly search = signal('');
  protected readonly queueFilter = signal('');
  protected readonly page = signal(1);
  protected readonly pageSize = signal(25);
  /** Keyed by channel — the same value pickUp/sendRedirect act on. */
  protected readonly redirectingChannel = signal<string | null>(null);
  protected readonly redirectTarget = signal('');

  private queueInputSeeded = false;

  protected readonly filtered = computed(() => {
    const q = this.search().toLowerCase().trim();
    const queue = this.queueFilter();
    let list = this.phone.queueWaitingCalls();
    if (queue) {
      list = list.filter((c) => c.queue === queue);
    }
    if (q) {
      list = list.filter(
        (c) => c.caller_id.toLowerCase().includes(q) || c.queue.toLowerCase().includes(q),
      );
    }
    return list;
  });

  protected readonly paginated = computed(() => {
    const start = (this.page() - 1) * this.pageSize();
    return this.filtered().slice(start, start + this.pageSize());
  });

  constructor() {
    // Seed the queue-login input from the agent's assigned queues the first
    // time bootstrap arrives — after that it's the agent's own text to edit,
    // never overwritten from underneath them.
    effect(() => {
      const bootstrap = this.phone.bootstrap();
      if (bootstrap && !this.queueInputSeeded) {
        this.queueInput.set(bootstrap.queues.join('|'));
        this.queueInputSeeded = true;
      }
    });
  }

  protected login(): void {
    this.phone.queueLogin(this.queueInput());
  }

  protected logout(): void {
    this.phone.queueLogout(this.queueInput());
  }

  protected togglePause(): void {
    if (this.phone.queuePaused()) {
      this.phone.queueUnpause();
    } else {
      this.phone.queuePause();
    }
  }

  protected refresh(): void {
    this.phone.refreshLiveQueues();
    this.phone.refreshLiveAgents();
  }

  protected wait(seconds: number): string {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  }

  protected pickUp(channel: string): void {
    const extension = this.phone.bootstrap()?.extension;
    if (extension) {
      this.phone.pickupQueueCall(channel, extension);
    }
  }

  protected startRedirect(channel: string): void {
    this.redirectingChannel.set(channel);
    this.redirectTarget.set('');
  }

  protected sendRedirect(channel: string): void {
    const target = this.redirectTarget().trim();
    if (target) {
      this.phone.redirectQueueCall(channel, target);
    }
    this.redirectingChannel.set(null);
  }
}
