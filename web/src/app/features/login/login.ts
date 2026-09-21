import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';

import { AuthService } from '../../core/auth/auth.service';
import { IconComponent } from '../../shared/components/icon/icon';

/**
 * The way in.
 *
 * It renders its own inline error rather than relying on the global
 * notification dialog — the error interceptor deliberately skips login for
 * that reason. A modal over a form saying "unauthorized" puts the message
 * somewhere other than the fields it is about.
 */
@Component({
  selector: 'app-login',
  standalone: true,
  imports: [FormsModule, IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="flex min-h-svh items-center justify-center p-6">
      <div class="w-full max-w-sm">
        <div class="mb-8 flex flex-col items-center text-center">
          <div
            class="bg-brand text-brand-foreground mb-3 flex aspect-square size-12 items-center justify-center rounded-lg"
          >
            <app-icon name="phone-call" [size]="24" />
          </div>
          <h1 class="text-xl font-semibold tracking-tight">Call Center</h1>
          <p class="text-muted-foreground text-sm">Sign in to continue.</p>
        </div>

        <form class="card grid gap-4 p-6" (ngSubmit)="submit()">
          <div class="grid gap-1.5">
            <label for="username" class="text-sm font-medium">Username</label>
            <input
              id="username"
              name="username"
              class="input"
              autocomplete="username"
              required
              [disabled]="busy()"
              [(ngModel)]="username"
            />
          </div>

          <div class="grid gap-1.5">
            <label for="password" class="text-sm font-medium">Password</label>
            <input
              id="password"
              name="password"
              type="password"
              class="input"
              autocomplete="current-password"
              required
              [disabled]="busy()"
              [(ngModel)]="password"
            />
          </div>

          @if (error(); as message) {
            <p
              class="border-destructive/40 bg-destructive/10 text-destructive rounded-md border px-3 py-2 text-sm"
              role="alert"
            >
              {{ message }}
            </p>
          }

          <button type="submit" class="btn btn-default w-full" [disabled]="busy()">
            {{ busy() ? 'Signing in…' : 'Sign in' }}
          </button>
        </form>
      </div>
    </div>
  `,
})
export class LoginComponent {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  protected username = '';
  protected password = '';
  protected readonly busy = signal(false);
  protected readonly error = signal<string | null>(null);

  protected submit(): void {
    if (this.busy()) {
      return;
    }
    if (!this.username.trim() || !this.password) {
      this.error.set('Enter your username and password.');
      return;
    }

    this.busy.set(true);
    this.error.set(null);

    this.auth.login(this.username.trim(), this.password).subscribe({
      next: () => {
        this.busy.set(false);
        // Honour a returnUrl only when it is a same-site path: a value
        // starting with `//` is an absolute URL in disguise and would make
        // this an open redirect.
        const returnUrl = new URLSearchParams(window.location.search).get('returnUrl');
        const target =
          returnUrl && returnUrl.startsWith('/') && !returnUrl.startsWith('//')
            ? returnUrl
            : '/dashboard';
        void this.router.navigateByUrl(target);
      },
      error: (err: unknown) => {
        this.busy.set(false);
        const body = (err as { error?: { message?: string } })?.error;
        this.error.set(body?.message ?? 'Could not sign in. Please try again.');
      },
    });
  }
}
