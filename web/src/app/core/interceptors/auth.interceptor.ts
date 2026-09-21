import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';

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

  if (isOwnApi && !isLogin && token) {
    req = req.clone({ setHeaders: { Authorization: `Bearer ${token}` } });
  }

  return next(req);
};
