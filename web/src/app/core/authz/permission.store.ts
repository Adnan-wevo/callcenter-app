import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, catchError, finalize, map, of, shareReplay, tap, timeout } from 'rxjs';

import { ApiService } from '../api/api.service';
import { Permission } from './permissions';

/**
 * - `idle`    — nothing loaded yet (signed out, or before the first request).
 * - `loading` — an authority request is in flight.
 * - `ready`   — authority is loaded; `can()` answers from real data.
 * - `error`   — the authority read failed. The shell shows a retryable screen
 *               and `can()` denies everything. There is NO permissive
 *               default: a store that could not load its authority refuses,
 *               it never guesses.
 */
export type PermissionState = 'idle' | 'loading' | 'ready' | 'error';

export interface Authority {
  user_id: string;
  username: string;
  roles: string[];
  permissions: string[];
  super: boolean;
  allowed_queues: string[];
  allowed_agents: string[];
}

/**
 * Holds the caller's resolved authority and is the ONLY thing this app
 * consults to decide what a user may do.
 *
 * Authority is sourced from GET secure/me/authority — never from the JWT,
 * which carries identity only — and is re-resolvable on demand (sign-in, a
 * 403 resync, a manual retry).
 *
 * Fail-closed, always: `can()` returns true only when the state is `ready`.
 * While loading, on error, or before any load, every permission is denied.
 * That mirrors the server, where a missing permission is a refusal.
 */
@Injectable({ providedIn: 'root' })
export class PermissionStore {
  private readonly api = inject(ApiService);

  private readonly _state = signal<PermissionState>('idle');
  private readonly _permissions = signal<ReadonlySet<string>>(new Set());
  private readonly _authority = signal<Authority | null>(null);

  readonly state = this._state.asReadonly();
  readonly authority = this._authority.asReadonly();
  readonly ready = computed(() => this._state() === 'ready');
  readonly username = computed(() => this._authority()?.username ?? '');
  readonly roles = computed(() => this._authority()?.roles ?? []);

  private inFlight: Observable<PermissionState> | null = null;
  private resyncing = false;

  /**
   * Ensure authority is loaded, returning the terminal state once. Already
   * loaded (or errored) emits immediately; a load in flight is joined rather
   * than duplicated, so concurrent guards trigger exactly one request.
   */
  ensureLoaded(): Observable<PermissionState> {
    const s = this._state();
    if (s === 'ready' || s === 'error') {
      return of(s);
    }
    if (this.inFlight) {
      return this.inFlight;
    }
    return this.load();
  }

  /** Force a fresh load regardless of current state (sign-in, manual retry). */
  reload(): Observable<PermissionState> {
    this.inFlight = null;
    return this.load();
  }

  private load(): Observable<PermissionState> {
    this._state.set('loading');

    const shared = this.api.get<Authority>('secure/me/authority').pipe(
      // A request that connects but never answers must not wedge the store on
      // `loading` forever — that leaves the shell on a bare spinner with no
      // retry. Bound it so a stall falls into the error path instead.
      timeout(15000),
      tap((authority) => {
        this._authority.set(authority);
        this._permissions.set(new Set(authority.permissions ?? []));
      }),
      map(() => {
        this._state.set('ready');
        return 'ready' as PermissionState;
      }),
      catchError(() => {
        this.reset();
        this._state.set('error');
        return of('error' as PermissionState);
      }),
      finalize(() => {
        this.inFlight = null;
      }),
      shareReplay({ bufferSize: 1, refCount: false }),
    );

    this.inFlight = shared;
    return shared;
  }

  /**
   * Re-resolve authority after a 403, at most once at a time. The server is
   * the authority and it may have changed a grant out from under a cached
   * verdict; one refetch realigns the store so the UI stops offering a
   * control the server now refuses.
   */
  resyncAfterForbidden(): void {
    if (this.resyncing || this._state() !== 'ready') {
      return;
    }
    this.resyncing = true;
    this.api
      .get<Authority>('secure/me/authority')
      .pipe(finalize(() => (this.resyncing = false)))
      .subscribe({
        next: (authority) => {
          this._authority.set(authority);
          this._permissions.set(new Set(authority.permissions ?? []));
        },
        error: () => {
          /* keep the existing set; the server keeps enforcing regardless */
        },
      });
  }

  /** Wipe all authority on sign-out. */
  clear(): void {
    this.inFlight = null;
    this.resyncing = false;
    this._state.set('idle');
    this.reset();
  }

  private reset(): void {
    this._permissions.set(new Set());
    this._authority.set(null);
  }

  /**
   * Whether the caller holds `perm`. Denies unless ready; a super-admin holds
   * everything by short-circuit. Reads signals, so anything in a computed or
   * an effect re-evaluates when authority changes.
   */
  can(perm: Permission): boolean {
    if (this._state() !== 'ready') {
      return false;
    }
    if (this._authority()?.super) {
      return true;
    }
    return this._permissions().has(perm);
  }

  /** True when the caller holds ANY of the given permissions. */
  canAny(...perms: Permission[]): boolean {
    return perms.some((p) => this.can(p));
  }
}
