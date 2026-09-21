import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';

import { AuthService } from './auth.service';

/** Redirect to /login when there is no valid token. */
export const authGuard: CanActivateFn = (_route, state) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  if (auth.hasValidToken()) {
    return true;
  }

  // Not authenticated: drop the (possibly expired) token and send to login,
  // remembering where the user was headed.
  auth.logout();
  return router.createUrlTree(['/login'], { queryParams: { returnUrl: state.url } });
};

/**
 * Keep an already-signed-in caller off the login page.
 *
 * Somebody with a live session who lands on /login — by bookmark, by the back
 * button after signing in, or by typing it — wants the app, not a form asking
 * them to prove who they already are.
 *
 * It honours a returnUrl if present, because authGuard adds one when it
 * bounces a deep link here: a caller who followed a protected link and turns
 * out to already be signed in should land on the link they wanted.
 */
export const guestGuard: CanActivateFn = (route) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  if (!auth.hasValidToken()) {
    return true;
  }

  const returnUrl = route.queryParamMap.get('returnUrl');
  // Only same-site paths: a returnUrl starting with `//` is an absolute URL
  // in disguise and would turn this into an open redirect.
  if (returnUrl && returnUrl.startsWith('/') && !returnUrl.startsWith('//')) {
    return router.parseUrl(returnUrl);
  }

  return router.createUrlTree(['/dashboard']);
};
