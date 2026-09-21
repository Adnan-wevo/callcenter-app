import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { RouterLink } from '@angular/router';

import { IconComponent } from '../../shared/components/icon/icon';

export type ErrorKind = 'forbidden' | 'not-found';

/**
 * One component for both navigation errors. The kind arrives as route `data`
 * bound straight to the input (withComponentInputBinding), so neither route
 * needs a component of its own.
 *
 * A 403 is deliberately named rather than redirected away: bouncing somebody
 * who clicked a link they cannot use to the dashboard leaves them wondering
 * why they are suddenly on the home page.
 */
@Component({
  selector: 'app-error-page',
  standalone: true,
  imports: [RouterLink, IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="flex min-h-svh items-center justify-center p-6">
      <div class="w-full max-w-md text-center">
        <div
          class="bg-muted text-muted-foreground mx-auto mb-4 flex size-12 items-center justify-center rounded-full"
        >
          <app-icon name="alert" [size]="24" />
        </div>
        <h1 class="text-lg font-semibold">{{ title() }}</h1>
        <p class="text-muted-foreground mt-2 text-sm">{{ description() }}</p>
        <div class="mt-6 flex justify-center">
          <a routerLink="/dashboard" class="btn btn-default">Back to dashboard</a>
        </div>
      </div>
    </div>
  `,
})
export class ErrorPageComponent {
  readonly kind = input<ErrorKind>('not-found');

  protected readonly title = computed(() =>
    this.kind() === 'forbidden' ? 'You cannot open this' : 'Page not found',
  );

  protected readonly description = computed(() =>
    this.kind() === 'forbidden'
      ? 'You are signed in, but your account does not hold the permission this screen needs. If you think it should, ask an administrator.'
      : 'That link does not lead anywhere. It may have been moved, or mistyped.',
  );
}
