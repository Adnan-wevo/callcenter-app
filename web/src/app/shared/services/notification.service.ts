import { Injectable, signal } from '@angular/core';

export type NoteKind = 'success' | 'error' | 'info';

export interface Note {
  kind: NoteKind;
  message: string;
}

/**
 * The one place feedback appears.
 *
 * It is exactly ONE slot, modal, and it does not dismiss on navigation —
 * so a message is never lost before it is read. The kind changes the title,
 * the icon and the colour, and nothing else.
 */
@Injectable({ providedIn: 'root' })
export class NotificationService {
  private readonly _current = signal<Note | null>(null);

  readonly current = this._current.asReadonly();
  readonly isOpen = signal(false);

  success(message: string): void {
    this.show({ kind: 'success', message });
  }

  error(message: string): void {
    this.show({ kind: 'error', message });
  }

  info(message: string): void {
    this.show({ kind: 'info', message });
  }

  dismiss(): void {
    this.isOpen.set(false);
    this._current.set(null);
  }

  private show(note: Note): void {
    this._current.set(note);
    this.isOpen.set(true);
  }
}
