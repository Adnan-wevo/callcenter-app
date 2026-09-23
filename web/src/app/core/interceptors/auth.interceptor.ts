import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { throwError } from 'rxjs';

import { environment } from '../../environments/environment';
import { AuthService } from '../auth/auth.service';

/**
 * Attaches `Authorization: Bearer <token>` to calls going to THIS service's
 * API, and to nothing else.
 *
 * The origin check is the point: an interceptor that attaches the token to
 * every outgoing request will hand the user's credential to any third-party
 * URL the app ever fetches. Scoping it to the configured API base means the
 * token only travels where it is meant to.
 *
 * The login route is excluded because a caller reaching it holds nothing yet,
 * and a stale token there would be sent where a credential is not expected.
 */
export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);

  const isOwnApi = req.url.startsWith(`${environment.apiUrl}/api/`);
  const isLogin = req.url.includes('/open/auth/login');
  const token = auth.getToken();

  if (isOwnApi && !isLogin && !token) {
    // Signing out is not instant from the DOM's point of view. Clearing the
    // token re-renders the shell, and app.html's signed-out branch holds a
    // DIFFERENT <router-outlet> than the signed-in one, so Angular builds a
    // fresh outlet and re-instantiates whichever protected route is still in
    // the URL — the navigation to /login has not landed yet. On the dashboard
    // that means its constructor fires another summary request a tick after
    // the credential is gone. It could only ever 401, and the error
    // interceptor would put "authentication required" on screen over the
    // login form the user was just sent to.
    //
    // A request to this service's secure surface with nothing to
    // authenticate it is never going to succeed, so fail it here instead of
    // spending a round trip to be told so. Shaped as a 401 so anything
    // already handling expiry keeps working unchanged; it is not passed to
    // next(), so it never reaches the network — and since this interceptor
    // is registered OUTSIDE the error one, that also means the error
    // interceptor never sees it and puts no dialog on screen.
    //
    // The trade: the error interceptor's own 401 handling (sign out, bounce
    // to /login) no longer runs for these. That only matters when a token
    // disappears WITHOUT a sign-out — hand-cleared localStorage, say — where
    // the page now sits inert until the next navigation instead of
    // redirecting immediately. authGuard still catches it there.
    return throwError(
      () =>
        new HttpErrorResponse({
          status: 401,
          statusText: 'Unauthorized',
          url: req.url,
          error: { message: 'not signed in' },
        }),
    );
  }

  if (isOwnApi && !isLogin && token) {
    req = req.clone({ setHeaders: { Authorization: `Bearer ${token}` } });
  }

  return next(req);
};
