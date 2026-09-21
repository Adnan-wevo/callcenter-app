import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';

import { NotificationService } from '../../shared/services/notification.service';
import { AuthService } from '../auth/auth.service';
import { PermissionStore } from '../authz/permission.store';

/**
 * One place for what every failed request means.
 *
 * - **401** — clear the token and go to /login.
 * - **403** — re-resolve authority once. A forbidden response means the
 *   server's live verdict disagrees with what the UI offered (a grant may
 *   have been revoked out from under a stale set), so the store refetches to
 *   realign and stops offering a control the server now refuses. It never
 *   widens authority: the refetch IS the authority.
 * - **422** — pass through untouched. The body names the fields the backend
 *   rejected, ApiService turns it into an ApiValidationError, and the form
 *   renders it next to the inputs. A global dialog would only obscure them.
 * - **everything else** — surface the envelope's `message` in the one
 *   notification slot.
 */
export const errorInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);
  const router = inject(Router);
  const perms = inject(PermissionStore);
  const notify = inject(NotificationService);

  return next(req).pipe(
    catchError((err: HttpErrorResponse) => {
      if (err.status === 401) {
        auth.logout();
        // Avoid a redirect loop when the failing call WAS the sign-in attempt.
        if (!req.url.includes('/open/auth/login')) {
          void router.navigate(['/login']);
        }
      }

      // The authority route is mounted behind authentication alone and never
      // 403s, so this cannot loop. The path guard is belt-and-braces.
      if (err.status === 403 && !req.url.includes('/me/authority')) {
        perms.resyncAfterForbidden();
      }

      if (err.status === 422) {
        return throwError(() => err);
      }

      const envelope = err.error as { message?: string } | string | null;
      let message: string;
      if (envelope && typeof envelope === 'object' && typeof envelope.message === 'string') {
        message = envelope.message;
      } else if (typeof envelope === 'string' && envelope.length > 0) {
        message = envelope;
      } else if (err.status === 0) {
        message = 'Network error — the server is unreachable.';
      } else {
        message = err.message || `Request failed (${err.status}).`;
      }

      // The login page renders its own inline error, so skip the global
      // dialog there to avoid saying the same thing twice.
      if (!req.url.includes('/open/auth/login')) {
        notify.error(message);
      }

      return throwError(() => err);
    }),
  );
};
