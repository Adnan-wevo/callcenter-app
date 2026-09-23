import { Routes } from '@angular/router';

export const USER_FILTERS_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./user-filters-list/user-filters-list').then((m) => m.UserFiltersListComponent),
  },
];
