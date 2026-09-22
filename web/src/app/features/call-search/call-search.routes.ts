import { Routes } from '@angular/router';

export const CALL_SEARCH_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () =>
      import('./call-search-list/call-search-list').then((m) => m.CallSearchListComponent),
  },
];
