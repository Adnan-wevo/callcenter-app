import { Routes } from '@angular/router';

export const SIP_EXTENSIONS_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./sip-extensions-list/sip-extensions-list').then((m) => m.SipExtensionsListComponent),
  },
];
