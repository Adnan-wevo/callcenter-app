import { ApplicationConfig, provideBrowserGlobalErrorListeners } from '@angular/core';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { provideRouter, withComponentInputBinding } from '@angular/router';

import { routes } from './app.routes';
import { authInterceptor } from './core/interceptors/auth.interceptor';
import { errorInterceptor } from './core/interceptors/error.interceptor';
import { PermissionStore } from './core/authz/permission.store';
import { NAV_AUTHORITY } from './layout/nav';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    // withComponentInputBinding lets a route's `data` bind straight to a
    // component input, so one error component can serve 403 and 404 without
    // reading the ActivatedRoute by hand.
    provideRouter(routes, withComponentInputBinding()),
    // Order matters: authInterceptor attaches the credential, errorInterceptor
    // wraps the response. Reversing them would mean a 401 handler that runs
    // before the token was ever added.
    provideHttpClient(withInterceptors([authInterceptor, errorInterceptor])),
    // The sidebar resolves its entries against whatever authority is provided
    // where it is rendered. At the root that is the permission store.
    { provide: NAV_AUTHORITY, useExisting: PermissionStore },
  ],
};
