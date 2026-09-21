import { ChangeDetectionStrategy, Component, input } from '@angular/core';

/**
 * The heading block every screen opens with: a title, a one-line description
 * of what the screen is, and an optional row of page-level actions below it.
 *
 * Actions sit BELOW the title rather than beside it: sharing one row makes
 * the buttons compete with the heading for width and run off the edge of a
 * narrow viewport, where on their own row they wrap instead.
 *
 * `hasActions` is a separate input rather than detecting projected content,
 * because the row must not render — not even as empty padding — when every
 * action on the page is gated away from the caller.
 */
@Component({
  selector: 'app-page-header',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'block shrink-0' },
  template: `
    <div class="min-w-0">
      <h1 class="text-2xl font-semibold tracking-tight">{{ title() }}</h1>
      @if (description(); as desc) {
        <p class="text-muted-foreground text-sm">{{ desc }}</p>
      }
    </div>
    @if (hasActions()) {
      <div class="mt-4 flex flex-wrap items-center gap-2">
        <ng-content />
      </div>
    }
  `,
})
export class PageHeaderComponent {
  readonly title = input.required<string>();
  readonly description = input<string | null>(null);
  readonly hasActions = input(false);
}
