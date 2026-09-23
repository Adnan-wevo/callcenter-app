import { Routes } from '@angular/router';

export const USERS_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () => import('./users-list/users-list').then((m) => m.UsersListComponent),
  },
];
