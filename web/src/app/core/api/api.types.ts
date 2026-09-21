/**
 * The wire shapes of this service's response envelope.
 *
 * Every response is one of these two objects and the discriminator is
 * `status`, not the HTTP status code, so a client can branch on the body
 * alone. See internal/apires on the Go side.
 */

/** Pagination counters that ride with a collection response. */
export interface ApiMeta {
  page: number;
  per_page: number;
  total: number;
  last_page: number;
  search?: string;
  sort?: string;
  order?: 'asc' | 'desc';
}

export interface ApiEnvelope<T> {
  status: 'success';
  data: T;
  meta?: ApiMeta;
  message?: string;
}

/**
 * `errors` is populated on 422 alone and maps a field name to one or more
 * complaints. It is the only failure body that carries detail, and the
 * detail is always about the request the client sent.
 */
export interface ApiErrorEnvelope {
  status: 'error';
  message: string;
  code: number;
  errors?: Record<string, string[]>;
}

/** A page of rows plus the counters describing it. */
export interface Page<T> {
  rows: T[];
  meta: ApiMeta;
}

/**
 * A 422 turned into something a form can consume. Thrown by ApiService so a
 * caller that cares about field errors can catch this one type, while callers
 * that do not keep seeing an ordinary HttpErrorResponse.
 */
export class ApiValidationError extends Error {
  constructor(
    readonly fields: Record<string, string[]>,
    message = 'Validation failed',
  ) {
    super(message);
    this.name = 'ApiValidationError';
  }

  /** The first complaint recorded for `field`, or undefined. */
  first(field: string): string | undefined {
    return this.fields[field]?.[0];
  }
}
