import { ChangeDetectionStrategy, Component, computed, effect, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterOutlet } from '@angular/router';
import { filter, map, startWith } from 'rxjs';

import { AuthService } from './core/auth/auth.service';
import { PermissionStore } from './core/authz/permission.store';
import { CALLCENTER_BRAND, CALLCENTER_NAV } from './layout/callcenter-nav';
import { SidebarComponent } from './layout/callcenter-sidebar';
import { IconComponent } from './shared/components/icon/icon';
import { NotificationService } from './shared/services/notification.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, SidebarComponent, IconComponent],
  templateUrl: './app.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class App {
  private readonly auth = inject(AuthService);
  private readonly perms = inject(PermissionStore);
  private readonly router = inject(Router);
  protected readonly notify = inject(NotificationService);

  protected readonly brand = CALLCENTER_BRAND;
  protected readonly navGroups = CALLCENTER_NAV;

  protected readonly isAuthenticated = this.auth.isAuthenticated;
  protected readonly username = this.perms.username;
  protected readonly roles = this.perms.roles;

  /**
   * The shell renders against this: 'ready' shows the app, 'error' shows the
   * retry screen, anything else shows the loading state. Authority is NEVER
   * assumed on failure — a non-ready state is a gate, not a fall-through.
   */
  protected readonly permState = this.perms.state;

  private readonly currentUrl = toSignal(
    this.router.events.pipe(
      filter((e): e is NavigationEnd => e instanceof NavigationEnd),
      map((e) => e.urlAfterRedirects),
      startWith(this.router.url),
    ),
    { initialValue: this.router.url },
  );

  /** Login renders bare — there is no session to hang a shell on yet. */
  protected readonly isBare = computed(() => this.currentUrl().startsWith('/login'));

  constructor() {
    // Non-blocking bootstrap: whenever there is a valid session, make sure
    // authority is loaded. An effect rather than a blocking initializer, so
    // the shell can react to loading/error/ready instead of freezing the
    // whole app on one round trip.
    effect(() => {
      if (this.isAuthenticated()) {
        this.perms.ensureLoaded().subscribe();
      }
    });
  }

  protected retryPermissions(): void {
    this.perms.reload().subscribe();
  }

  protected signOut(): void {
    this.auth.logout();
    void this.router.navigate(['/login']);
  }
}
