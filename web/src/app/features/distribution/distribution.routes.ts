import { Routes } from '@angular/router';

export const DISTRIBUTION_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./distribution-page/distribution-page').then((m) => m.DistributionPageComponent),
  },
];
