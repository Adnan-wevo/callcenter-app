import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { ApiService } from '../../core/api/api.service';
import { Page } from '../../core/api/api.types';

/** One row of sip_extensions, as internal/softphone.ListItem returns it —
 * never a password, this service has no directory to resolve `user_id`
 * against on its own (see ListUsers below). */
export interface SipExtensionRow {
  id: string;
  user_id: string;
  extension: string;
  display_name: string;
  is_supervisor: boolean;
  is_default: boolean;
  queues: string[];
}

/** Mirrors handlers.publicUser — enough to label `user_id` in the UI and
 * populate the "assign to" picker, nothing credential-shaped. */
export interface SipUser {
  id: string;
  username: string;
  roles: string[];
}

/** Mirrors handlers.sipExtensionBody. `queues` is raw text here (e.g.
 * "40000|40001"), not an array — the server normalises it
 * (internal/softphone/repository.go's normalizeQueues), matching the same
 * pipe/comma/space-separated convention the softphone Queue tab's own
 * login input uses. */
export interface SipExtensionInput {
  user_id?: string;
  extension: string;
  sip_username: string;
  sip_password?: string;
  display_name?: string;
  queues?: string;
  is_supervisor?: boolean;
  is_default?: boolean;
}

@Injectable({ providedIn: 'root' })
export class SipExtensionsService {
  private readonly api = inject(ApiService);

  list(params?: Record<string, unknown>): Observable<Page<SipExtensionRow>> {
    return this.api.list<SipExtensionRow>('secure/admin/sip-extensions', params);
  }

  users(): Observable<SipUser[]> {
    return this.api.get<SipUser[]>('secure/admin/users');
  }

  create(body: SipExtensionInput): Observable<{ id: string }> {
    return this.api.post<{ id: string }>('secure/admin/sip-extensions', body);
  }

  update(id: string, body: SipExtensionInput): Observable<{ status: string }> {
    return this.api.put<{ status: string }>(`secure/admin/sip-extensions/${id}`, body);
  }

  delete(id: string): Observable<{ status: string }> {
    return this.api.delete<{ status: string }>(`secure/admin/sip-extensions/${id}`);
  }
}
