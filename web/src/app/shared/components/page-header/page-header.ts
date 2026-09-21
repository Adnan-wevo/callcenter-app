import { ChangeDetectionStrategy, Component, input } from '@angular/core';

import { IconComponent } from '../icon/icon';

/**
 * The heading block every screen opens with, matching heal-crm's own:
 * a coloured icon beside a bold title, a one-line description under it, and
 * a rule closing the block off.
 *
 * The icon is not decoration — heal-crm colours it per screen (a red
 * phone-x-mark on Unanswered Calls, for instance), so a user recognises
 * which report they are on before reading the title.
 *
 * `hasActions` is a separate input rather than detecting projected content,
 * because the row must not render — not even as empty padding — when every
 * action on the page is gated away from the caller.
 */
@Component({
  selector: 'app-page-header',
  standalone: true,
  imports: [IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'block shrink-0' },
  template: `
    <div
      class="flex flex-col gap-3 border-b pb-4 sm:flex-row sm:items-start sm:justify-between"
    >
      <div class="flex flex-col gap-1">
        <div class="flex items-center gap-2">
          @if (icon(); as name) {
            <span [class]="iconClass()">
              <app-icon [name]="name" [size]="20" />
            </span>
          }
          <h1 class="text-xl font-bold tracking-tight">{{ title() }}</h1>
        </div>
        @if (description(); as desc) {
          <p class="text-muted-foreground text-sm">{{ desc }}</p>
        }
      </div>

      @if (hasActions()) {
        <div class="flex flex-wrap items-center gap-2">
          <ng-content />
        </div>
      }
    </div>
  `,
})
export class PageHeaderComponent {
  readonly title = input.required<string>();
  readonly description = input<string | null>(null);
  readonly icon = input<string | null>(null);
  /** Tailwind colour classes for the icon, e.g. 'text-red-500'. */
  readonly iconClass = input('text-brand');
  readonly hasActions = input(false);
}
