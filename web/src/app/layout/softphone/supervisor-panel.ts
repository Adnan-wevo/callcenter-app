import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';

import { SoftphoneService, SpyMode } from '../../core/softphone/softphone.service';
import { IconComponent } from '../../shared/components/icon/icon';

/**
 * Monitor / Whisper / Barge — visible only to a supervisor extension
 * (`bootstrap.is_supervisor`), matching heal-crm's own
 * `livewire/partials/supervisor-panel.blade.php`: the actions this panel
 * offers exist only server-side for someone the server itself will accept
 * as a supervisor (`softphone.supervise`, see router.go) — this component
 * is the entry point, not the gate.
 *
 * heal-crm shows these as buttons that open a target-picker MODAL; this app
 * has no modal primitive yet, so the picker is an inline list under
 * whichever button was clicked instead — same choice (pick a live agent,
 * then spy), simpler chrome.
 */
@Component({
  selector: 'app-supervisor-panel',
  standalone: true,
  imports: [IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (phone.bootstrap()?.is_supervisor && !inCall()) {
      <div class="card mb-3 p-2.5">
        <div class="text-muted-foreground mb-2 flex items-center gap-1.5 text-xs font-semibold tracking-wide uppercase">
          <app-icon name="shield" [size]="13" />
          Supervisor
        </div>
        <div class="grid grid-cols-3 gap-1.5">
          <button type="button" class="btn btn-outline btn-sm gap-1 px-2 text-xs" [class]="openMode() === 'monitor' ? '!bg-accent' : ''" title="Silent monitor — listen to an agent's call" (click)="toggle('monitor')">
            <app-icon name="headphones" [size]="14" />
            Monitor
          </button>
          <button type="button" class="btn btn-outline btn-sm gap-1 px-2 text-xs" [class]="openMode() === 'whisper' ? '!bg-accent' : ''" title="Whisper — talk to the agent only, caller can't hear" (click)="toggle('whisper')">
            <app-icon name="volume-2" [size]="14" />
            Whisper
          </button>
          <button type="button" class="btn btn-outline btn-sm gap-1 px-2 text-xs" [class]="openMode() === 'barge' ? '!bg-accent' : ''" title="Barge — join the call as a third party" (click)="toggle('barge')">
            <app-icon name="users" [size]="14" />
            Barge
          </button>
        </div>

        @if (openMode(); as mode) {
          <div class="mt-2 max-h-40 overflow-y-auto rounded-md border">
            @if (phone.agentsOnCall().length === 0) {
              <p class="text-muted-foreground px-2 py-3 text-center text-xs">No agents are on a call right now.</p>
            } @else {
              @for (agent of phone.agentsOnCall(); track agent.extension) {
                <button
                  type="button"
                  class="hover:bg-accent flex w-full items-center justify-between gap-2 border-b px-2 py-1.5 text-left text-xs last:border-0"
                  (click)="choose(agent.extension, mode)"
                >
                  <span class="font-medium">{{ agent.extension }}</span>
                  <span class="text-muted-foreground truncate">{{ agent.active_call?.caller_id }}</span>
                </button>
              }
            }
          </div>
        }
      </div>
    }
  `,
})
export class SupervisorPanelComponent {
  protected readonly phone = inject(SoftphoneService);

  protected readonly inCall = computed(() => this.phone.activeCalls().length > 0);
  protected readonly openMode = signal<SpyMode | null>(null);

  protected toggle(mode: SpyMode): void {
    this.openMode.set(this.openMode() === mode ? null : mode);
  }

  protected choose(extension: string, mode: SpyMode): void {
    this.phone.spy(extension, mode);
    this.openMode.set(null);
  }
}
