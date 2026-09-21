import { Injectable, computed, inject, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, map, switchMap, tap } from 'rxjs';

import { environment } from '../../environments/environment';
import { ApiEnvelope } from '../api/api.types';
import { PermissionStore } from '../authz/permission.store';

const TOKEN_KEY = 'callcenter_token';

export interface TokenResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
}

/**
 * The JWT payload the Go service issues. It carries IDENTITY ONLY — no role,
 * no permission. Authority is never read from the token; it is resolved live
 * into the PermissionStore. Decoding here is limited to the subject and
 * expiry the client needs for its own session bookkeeping, and even those are
 * advisory — the server re-validates on every request.
 */
export interface JwtClaims {
  user_id: string;
  exp?: number;
  iat?: number;
}

/**
 * Decode a JWT payload WITHOUT verifying the signature. The client reads only
 * the subject and expiry; the backend is always the real authority for both
 * authentication and authorization. Returns null for a malformed token.
 */
export function decodeJwt(token: string): JwtClaims | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) {
      return null;
    }
    const payload = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const padded = payload.padEnd(payload.length + ((4 - (payload.length % 4)) % 4), '=');
    return JSON.parse(atob(padded)) as JwtClaims;
  } catch {
    return null;
  }
}

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly http = inject(HttpClient);
  private readonly perms = inject(PermissionStore);
  private readonly apiUrl = environment.apiUrl;

  // NOTE: localStorage is acceptable for this internal tool. It survives a
  // refresh and is only ever attached to this service's own API calls by the
  // auth interceptor.
  private readonly token = signal<string | null>(localStorage.getItem(TOKEN_KEY));

  readonly claims = computed<JwtClaims | null>(() => {
    const t = this.token();
    return t ? decodeJwt(t) : null;
  });

  readonly isAuthenticated = computed<boolean>(() => this.hasValidToken());

  /**
   * Authenticate, store the token, then resolve authority before completing.
   * The login component navigates only after this emits, so the first view
   * renders against a loaded permission store rather than an empty one.
   */
  login(username: string, password: string): Observable<TokenResponse> {
    return this.http
      .post<ApiEnvelope<TokenResponse>>(`${this.apiUrl}/api/v1/open/auth/login`, {
        username,
        password,
      })
      .pipe(
        map((res) => res.data),
        tap((data) => {
          if (data?.access_token) {
            this.setToken(data.access_token);
          }
        }),
        switchMap((data) => this.perms.reload().pipe(map(() => data))),
      );
  }

  logout(): void {
    localStorage.removeItem(TOKEN_KEY);
    this.token.set(null);
    this.perms.clear();
  }

  getToken(): string | null {
    return this.token();
  }

  setToken(token: string): void {
    localStorage.setItem(TOKEN_KEY, token);
    this.token.set(token);
  }

  hasValidToken(): boolean {
    const c = this.claims();
    if (!c) {
      return false;
    }
    if (typeof c.exp === 'number') {
      return c.exp * 1000 > Date.now();
    }
    return true;
  }
}
