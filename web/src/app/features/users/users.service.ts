import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';
import { Page } from '../../core/api/api.types';

/** Mirrors handlers.adminUser. Backed by pbx-worker v3's directory, which
 *  is also what sign-ins fall through to — a user created here can log in
 *  straight away. */
export interface UserRow {
  id: string;
  name: string;
  email: string;
  roles: string[];
}

export interface UserInput {
  name: string;
  email: string;
  /** Required on create, never sent on update — this app never handles an
   *  existing password, v3 owns it. */
  password?: string;
  role: string;
}

@Injectable({ providedIn: 'root' })
export class UsersService {
  private readonly api = inject(ApiService);

  list(params?: Record<string, unknown>): Observable<Page<UserRow>> {
    return this.api.list<UserRow>('secure/admin/users/manage', params);
  }

  create(body: UserInput): Observable<{ id: string }> {
    return this.api.post<{ id: string }>('secure/admin/users', body);
  }

  update(id: string, body: UserInput): Observable<{ status: string }> {
    return this.api.put<{ status: string }>(`secure/admin/users/${id}`, body);
  }

  delete(id: string): Observable<{ status: string }> {
    return this.api.delete<{ status: string }>(`secure/admin/users/${id}`);
  }

  /** The real assignable set, read from v3 rather than hardcoded here so the
   *  form cannot drift from what the directory actually accepts. */
  roles(): Observable<string[]> {
    return this.api.get<string[]>('secure/admin/roles');
  }
}
