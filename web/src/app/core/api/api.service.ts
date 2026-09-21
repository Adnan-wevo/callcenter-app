import { HttpClient, HttpErrorResponse, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, catchError, map, throwError } from 'rxjs';

import { environment } from '../../environments/environment';
import { ApiEnvelope, ApiErrorEnvelope, ApiValidationError, Page } from './api.types';

/**
 * The one place this app talks to the backend.
 *
 * Feature services hold their PATHS and their types and call through here, so
 * envelope-unwrapping is a rule rather than a convention each of them has to
 * remember, and behaviour that must apply to every call — turning a 422 into
 * typed field errors — has somewhere to live.
 *
 * Paths are passed WITHOUT the `/api/v1` prefix, e.g.
 * `secure/call-center/queues`.
 */
@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);
  private readonly base = `${environment.apiUrl}/api/v1`;

  /** GET a single resource and unwrap it. */
  get<T>(path: string, params?: Record<string, unknown>): Observable<T> {
    return this.http.get<ApiEnvelope<T>>(this.url(path), { params: toParams(params) }).pipe(
      map((envelope) => envelope.data),
      catchError(translate),
    );
  }

  /** GET a collection and return both the rows and the pagination counters. */
  list<T>(path: string, params?: Record<string, unknown>): Observable<Page<T>> {
    return this.http.get<ApiEnvelope<T[]>>(this.url(path), { params: toParams(params) }).pipe(
      map((envelope) => ({
        rows: envelope.data ?? [],
        meta: envelope.meta ?? {
          page: 1,
          per_page: envelope.data?.length ?? 0,
          total: envelope.data?.length ?? 0,
          last_page: 1,
        },
      })),
      catchError(translate),
    );
  }

  post<T>(path: string, body?: unknown): Observable<T> {
    return this.http.post<ApiEnvelope<T>>(this.url(path), body ?? {}).pipe(
      map((envelope) => envelope.data),
      catchError(translate),
    );
  }

  private url(path: string): string {
    return `${this.base}/${path.replace(/^\/+/, '')}`;
  }
}

/**
 * Turns a 422 into an ApiValidationError and re-throws everything else as it
 * arrived. The narrowing matters to the error interceptor: a 422 is a
 * statement about the form the user is filling in, so it belongs next to the
 * offending field and not in the global dialog.
 */
export function translate(err: unknown): Observable<never> {
  if (err instanceof HttpErrorResponse && err.status === 422) {
    const body = err.error as ApiErrorEnvelope | null;
    if (body && typeof body === 'object' && body.errors) {
      return throwError(() => new ApiValidationError(body.errors!, body.message));
    }
  }
  return throwError(() => err);
}

/**
 * Serialises query parameters, dropping empty ones so an unset filter does
 * not travel as the string "undefined" for the backend to parse.
 *
 * An array value is emitted as REPEATED keys (`queues=Sales&queues=Support`),
 * which is what the Go side's `c.QueryArray` reads.
 */
export function toParams(source?: Record<string, unknown>): HttpParams {
  let params = new HttpParams();
  if (!source) {
    return params;
  }

  for (const [key, value] of Object.entries(source)) {
    if (value === undefined || value === null || value === '') {
      continue;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        if (item !== undefined && item !== null && item !== '') {
          params = params.append(key, String(item));
        }
      }
      continue;
    }
    params = params.set(key, String(value));
  }

  return params;
}
